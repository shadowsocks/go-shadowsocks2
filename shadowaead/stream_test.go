package shadowaead

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"golang.org/x/crypto/chacha20poly1305"
)

func newTestCipher(t *testing.T) Cipher {
	key := []byte("12345678901234567890123456789012") // 32 bytes
	cipher, err := Chacha20Poly1305(key)
	if err != nil {
		t.Fatal(err)
	}
	return cipher
}

// Overhead for cipher chacha20poly1305
const testCipherOverhead = 16

func TestCipherReaderAuthentiationFailure(t *testing.T) {
	cipher := newTestCipher(t)

	clientReader := strings.NewReader("Fails Authentication")
	reader := NewShadowsocksReader(clientReader, cipher)
	_, err := reader.Read(make([]byte, 1))
	if err == nil {
		t.Fatalf("Expected authentication failure, got %v", err)
	}
}

func TestCipherReaderUnexpectedEOF(t *testing.T) {
	cipher := newTestCipher(t)

	clientReader := strings.NewReader("short")
	server := NewShadowsocksReader(clientReader, cipher)
	_, err := server.Read(make([]byte, 10))
	if err != io.ErrUnexpectedEOF {
		t.Fatalf("Expected ErrUnexpectedEOF, got %v", err)
	}
}

func TestCipherReaderEOF(t *testing.T) {
	cipher := newTestCipher(t)

	clientReader := strings.NewReader("")
	server := NewShadowsocksReader(clientReader, cipher)
	_, err := server.Read(make([]byte, 10))
	if err != io.EOF {
		t.Fatalf("Expected EOF, got %v", err)
	}
	_, err = server.Read([]byte{})
	if err != io.EOF {
		t.Fatalf("Expected EOF, got %v", err)
	}
}

func encryptBlocks(cipher Cipher, salt []byte, blocks [][]byte) (io.Reader, error) {
	var ssText bytes.Buffer
	aead, err := cipher.Encrypter(salt)
	if err != nil {
		return nil, fmt.Errorf("Failed to create AEAD: %v", err)
	}
	ssText.Write(salt)
	// buf must fit the larges block ciphertext
	buf := make([]byte, 2+100+testCipherOverhead)
	var expectedCipherSize int
	nonce := make([]byte, chacha20poly1305.NonceSize)
	for _, block := range blocks {
		ssText.Write(aead.Seal(buf[:0], nonce, []byte{0, byte(len(block))}, nil))
		nonce[0]++
		expectedCipherSize += 2 + testCipherOverhead
		ssText.Write(aead.Seal(buf[:0], nonce, block, nil))
		nonce[0]++
		expectedCipherSize += len(block) + testCipherOverhead
	}
	if ssText.Len() != cipher.SaltSize()+expectedCipherSize {
		return nil, fmt.Errorf("cipherText has size %v. Expected %v", ssText.Len(), cipher.SaltSize()+expectedCipherSize)
	}
	return &ssText, nil
}

func TestCipherReaderGoodReads(t *testing.T) {
	cipher := newTestCipher(t)

	salt := []byte("12345678901234567890123456789012")
	if len(salt) != cipher.SaltSize() {
		t.Fatalf("Salt has size %v. Expected %v", len(salt), cipher.SaltSize())
	}
	ssText, err := encryptBlocks(cipher, salt, [][]byte{
		[]byte("[First Block]"),
		[]byte(""), // Corner case: empty block
		[]byte("[Third Block]")})
	if err != nil {
		t.Fatal(err)
	}

	reader := NewShadowsocksReader(ssText, cipher)
	plainText := make([]byte, len("[First Block]")+len("[Third Block]"))
	n, err := io.ReadFull(reader, plainText)
	if err != nil {
		t.Fatalf("Failed to fully read plain text. Got %v bytes: %v", n, err)
	}
	_, err = reader.Read([]byte{})
	if err != io.EOF {
		t.Fatalf("Expected EOF, got %v", err)
	}
	_, err = reader.Read(make([]byte, 1))
	if err != io.EOF {
		t.Fatalf("Expected EOF, got %v", err)
	}
}

func TestCipherReaderClose(t *testing.T) {
	cipher := newTestCipher(t)

	pipeReader, pipeWriter := io.Pipe()
	server := NewShadowsocksReader(pipeReader, cipher)
	result := make(chan error)
	go func() {
		_, err := server.Read(make([]byte, 10))
		result <- err
	}()
	pipeWriter.Close()
	err := <-result
	if err != io.EOF {
		t.Fatalf("Expected ErrUnexpectedEOF, got %v", err)
	}
}

func TestCipherReaderCloseError(t *testing.T) {
	cipher := newTestCipher(t)

	pipeReader, pipeWriter := io.Pipe()
	server := NewShadowsocksReader(pipeReader, cipher)
	result := make(chan error)
	go func() {
		_, err := server.Read(make([]byte, 10))
		result <- err
	}()
	pipeWriter.CloseWithError(fmt.Errorf("xx!!ERROR!!xx"))
	err := <-result
	if err == nil || !strings.Contains(err.Error(), "xx!!ERROR!!xx") {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestEndToEnd(t *testing.T) {
	cipher := newTestCipher(t)

	connReader, connWriter := io.Pipe()
	writer := NewShadowsocksWriter(connWriter, cipher)
	reader := NewShadowsocksReader(connReader, cipher)
	expected := "Test"
	go func() {
		defer connWriter.Close()
		_, err := writer.Write([]byte(expected))
		if err != nil {
			t.Fatalf("Failed Write: %v", err)
		}
	}()
	var output bytes.Buffer
	_, err := reader.WriteTo(&output)
	if err != nil {
		t.Fatalf("Failed WriteTo: %v", err)
	}
	if output.String() != expected {
		t.Fatalf("Expected output '%v'. Got '%v'", expected, output.String())
	}
}
