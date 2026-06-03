package studios

import (
	"testing"
)

func TestChunkText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxChars int
		overlap  int
		expected int // minimum expected chunks
	}{
		{
			name:     "Short text no chunking",
			text:     "Hello world",
			maxChars: 50,
			overlap:  10,
			expected: 1,
		},
		{
			name:     "Long text simple split",
			text:     "The quick brown fox jumps over the lazy dog.",
			maxChars: 10,
			overlap:  3,
			expected: 6,
		},
		{
			name:     "Empty text",
			text:     "",
			maxChars: 10,
			overlap:  2,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := ChunkText(tt.text, tt.maxChars, tt.overlap)
			if len(chunks) != tt.expected {
				t.Errorf("ChunkText() got = %d chunks, want = %d", len(chunks), tt.expected)
			}
			// Verify all chunks are non-empty
			for _, c := range chunks {
				if c == "" {
					t.Errorf("ChunkText() produced an empty chunk")
				}
			}
		})
	}
}

func TestFormatVectorAsString(t *testing.T) {
	vec := []float32{0.1, -0.2, 0.3}
	got := FormatVectorAsString(vec)
	want := "[0.1,-0.2,0.3]"
	if got != want {
		t.Errorf("FormatVectorAsString() = %s, want %s", got, want)
	}
}
