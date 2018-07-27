package shadowaead

import (
	"bytes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"net"

	ssnet "github.com/shadowsocks/go-shadowsocks2/net"
)

// payloadSizeMask is the maximum size of payload in bytes.
const payloadSizeMask = 0x3FFF // 16*1024 - 1

// ShadowsocksWriter is an io.Writer that also implements io.ReaderFrom to
// allow for piping the data without extra allocations and copies.
type ShadowsocksWriter interface {
	io.Writer
	io.ReaderFrom
}

type shadowsocksWriter struct {
	writer   io.Writer
	ssCipher Cipher
	// These are lazily initialized:
	buf   []byte
	aead  cipher.AEAD
	nonce []byte
}

// NewShadowsocksWriter creates a Writer that encrypts the given Writer using
// the shadowsocks protocol with the given shadowsocks cipher.
func NewShadowsocksWriter(writer io.Writer, ssCipher Cipher) ShadowsocksWriter {
	return &shadowsocksWriter{writer: writer, ssCipher: ssCipher}
}

// init generates a random salt, sets up the AEAD object and writes
// the salt to the inner Writer.
func (sw *shadowsocksWriter) init() (err error) {
	if sw.aead == nil {
		salt := make([]byte, sw.ssCipher.SaltSize())
		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
			return fmt.Errorf("failed to generate salt: %v", err)
		}
		_, err := sw.writer.Write(salt)
		if err != nil {
			return fmt.Errorf("failed to write salt: %v", err)
		}
		sw.aead, err = sw.ssCipher.Encrypter(salt)
		if err != nil {
			return fmt.Errorf("failed to create AEAD: %v", err)
		}
		sw.nonce = make([]byte, sw.aead.NonceSize())
		sw.buf = make([]byte, 2+sw.aead.Overhead()+payloadSizeMask+sw.aead.Overhead())
	}
	return nil
}

// WriteBlock encrypts and writes the input buffer as one signed block.
func (sw *shadowsocksWriter) encryptBlock(ciphertext []byte, plaintext []byte) ([]byte, error) {
	out := sw.aead.Seal(ciphertext, sw.nonce, plaintext, nil)
	increment(sw.nonce)
	return out, nil
}

func (sw *shadowsocksWriter) Write(p []byte) (int, error) {
	n, err := sw.ReadFrom(bytes.NewReader(p))
	return int(n), err
}

func (sw *shadowsocksWriter) ReadFrom(r io.Reader) (int64, error) {
	if err := sw.init(); err != nil {
		return 0, err
	}
	var written int64
	sizeBuf := sw.buf[:2+sw.aead.Overhead()]
	payloadBuf := sw.buf[len(sizeBuf):]
	for {
		plaintextSize, err := r.Read(payloadBuf[:payloadSizeMask])
		if plaintextSize > 0 {
			// big-endian payload size
			sizeBuf[0], sizeBuf[1] = byte(plaintextSize>>8), byte(plaintextSize)
			_, err = sw.encryptBlock(sizeBuf[:0], sizeBuf[:2])
			if err != nil {
				return written, fmt.Errorf("failed to encypt payload size: %v", err)
			}
			_, err := sw.encryptBlock(payloadBuf[:0], payloadBuf[:plaintextSize])
			if err != nil {
				return written, fmt.Errorf("failed to encrypt payload: %v", err)
			}
			payloadSize := plaintextSize + sw.aead.Overhead()
			_, err = sw.writer.Write(sw.buf[:len(sizeBuf)+payloadSize])
			written += int64(plaintextSize)
		}
		if err != nil {
			if err == io.EOF { // ignore EOF as per io.ReaderFrom contract
				return written, nil
			}
			return written, fmt.Errorf("Failed to read payload: %v", err)
		}
	}
}

type shadowsocksReader struct {
	reader   io.Reader
	ssCipher Cipher
	// These are lazily initialized:
	aead     cipher.AEAD
	nonce    []byte
	buf      []byte
	leftover []byte
}

// ShadowsocksReader is an io.Reader that also implements io.WriterTo to
// allow for piping the data without extra allocations and copies.
type ShadowsocksReader interface {
	io.Reader
	io.WriterTo
}

// NewShadowsocksReader creates a Reader that decrypts the given Reader using
// the shadowsocks protocol with the given shadowsocks cipher.
func NewShadowsocksReader(reader io.Reader, ssCipher Cipher) ShadowsocksReader {
	return &shadowsocksReader{reader: reader, ssCipher: ssCipher}
}

// init reads the salt from the inner Reader and sets up the AEAD object
func (sr *shadowsocksReader) init() (err error) {
	if sr.aead == nil {
		salt := make([]byte, sr.ssCipher.SaltSize())
		if _, err := io.ReadFull(sr.reader, salt); err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				err = fmt.Errorf("failed to read salt: %v", err)
			}
			return err
		}
		sr.aead, err = sr.ssCipher.Decrypter(salt)
		if err != nil {
			return fmt.Errorf("failed to create AEAD: %v", err)
		}
		sr.nonce = make([]byte, sr.aead.NonceSize())
		sr.buf = make([]byte, payloadSizeMask+sr.aead.Overhead())
	}
	return nil
}

// ReadBlock reads and decrypts a single signed block of ciphertext.
// The block will match the given decryptedBlockSize.
// The returned slice is only valid until the next Read call.
func (sr *shadowsocksReader) readBlock(decryptedBlockSize int) ([]byte, error) {
	if err := sr.init(); err != nil {
		return nil, err
	}
	cipherBlockSize := decryptedBlockSize + sr.aead.Overhead()
	if cipherBlockSize > cap(sr.buf) {
		return nil, io.ErrShortBuffer
	}
	buf := sr.buf[:cipherBlockSize]
	_, err := io.ReadFull(sr.reader, buf)
	if err != nil {
		return nil, err
	}
	buf, err = sr.aead.Open(buf[:0], sr.nonce, buf, nil)
	increment(sr.nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %v", err)
	}
	return buf, nil
}

func (sr *shadowsocksReader) Read(b []byte) (int, error) {
	n, err := sr.readLoop(b)
	return int(n), err
}

func (sr *shadowsocksReader) WriteTo(w io.Writer) (written int64, err error) {
	n, err := sr.readLoop(w)
	if err == io.EOF {
		err = nil
	}
	return n, err
}

func (sr *shadowsocksReader) readLoop(w interface{}) (written int64, err error) {
	for {
		if len(sr.leftover) == 0 {
			buf, err := sr.readBlock(2)
			if err != nil {
				if err != io.EOF && err != io.ErrUnexpectedEOF {
					err = fmt.Errorf("failed to read payload size: %v", err)
				}
				return written, err
			}
			size := (int(buf[0])<<8 + int(buf[1])) & payloadSizeMask
			payload, err := sr.readBlock(size)
			if err != nil {
				return written, fmt.Errorf("failed to read payload: %v", err)
			}
			sr.leftover = payload
		}
		switch v := w.(type) {
		case io.Writer:
			n, err := v.Write(sr.leftover)
			written += int64(n)
			sr.leftover = sr.leftover[n:]
			if err != nil {
				return written, err
			}
		case []byte:
			n := copy(v, sr.leftover)
			sr.leftover = sr.leftover[n:]
			return int64(n), nil
		}
	}
}

// increment little-endian encoded unsigned integer b. Wrap around on overflow.
func increment(b []byte) {
	for i := range b {
		b[i]++
		if b[i] != 0 {
			return
		}
	}
}

// NewConn wraps a stream-oriented net.Conn with cipher.
func NewConn(c net.Conn, ciph Cipher) net.Conn {
	ssr := NewShadowsocksReader(c, ciph)
	ssw := NewShadowsocksWriter(c, ciph)
	return ssnet.WrapConn(c, ssr, ssw)
}
