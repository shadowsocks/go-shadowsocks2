package internal_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/lxt1045/go-shadowsocks2/internal"
)

var (
	bloomRingInstance *internal.BloomRing
)

func TestMain(m *testing.M) {
	bloomRingInstance = internal.NewBloomRing(internal.DefaultSFSlot, int(internal.DefaultSFCapacity),
		internal.DefaultSFFPR)
	os.Exit(m.Run())
}

func TestBloomRing_Add(t *testing.T) {
	defer func() {
		if any := recover(); any != nil {
			t.Fatalf("Should not got panic while adding item: %v", any)
		}
	}()
	bloomRingInstance.Add(make([]byte, 16))
}

func TestBloomRing_Test(t *testing.T) {
	buf := []byte("shadowsocks")
	bloomRingInstance.Add(buf)
	if !bloomRingInstance.Test(buf) {
		t.Fatal("Test on filter missing")
	}
}

func BenchmarkBloomRing(b *testing.B) {
	// Generate test samples with different length
	samples := make([][]byte, internal.DefaultSFCapacity-internal.DefaultSFSlot)
	var checkPoints [][]byte
	for i := 0; i < len(samples); i++ {
		samples[i] = []byte(fmt.Sprint(i))
		if i%1000 == 0 {
			checkPoints = append(checkPoints, samples[i])
		}
	}
	b.Logf("Generated %d samples and %d check points", len(samples), len(checkPoints))
	for i := 1; i < 16; i++ {
		b.Run(fmt.Sprintf("Slot%d", i), benchmarkBloomRing(samples, checkPoints, i))
	}
}

func benchmarkBloomRing(samples, checkPoints [][]byte, slot int) func(*testing.B) {
	filter := internal.NewBloomRing(slot, int(internal.DefaultSFCapacity), internal.DefaultSFFPR)
	for _, sample := range samples {
		filter.Add(sample)
	}
	return func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, cp := range checkPoints {
				filter.Test(cp)
			}
		}
	}
}
