package representation

import "testing"

var cgoSink int

func BenchmarkCgoAdapters(b *testing.B) {
	b.Run("Enum", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cgoSink = cgoModernValue(cgoInput.Value(i & 1023))
		}
	})
	b.Run("Tagged", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cgoSink = cgoTaggedValue(legacyCgoInput{true, i & 1023})
		}
	})
}
