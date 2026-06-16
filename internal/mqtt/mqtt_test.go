package mqtt

import "testing"

func TestEncodeRemainingLength(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want []byte
	}{
		{name: "one byte", in: 127, want: []byte{0x7f}},
		{name: "two bytes", in: 128, want: []byte{0x80, 0x01}},
		{name: "max", in: 268435455, want: []byte{0xff, 0xff, 0xff, 0x7f}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeRemainingLength(tt.in)
			if string(got) != string(tt.want) {
				t.Fatalf("encodeRemainingLength(%d)=% x want % x", tt.in, got, tt.want)
			}
		})
	}
}

func TestAppendString(t *testing.T) {
	got := appendString(nil, "abc")
	want := []byte{0x00, 0x03, 'a', 'b', 'c'}
	if string(got) != string(want) {
		t.Fatalf("appendString=% x want % x", got, want)
	}
}

func TestValidateTopic(t *testing.T) {
	for _, topic := range []string{"", "a/+", "a/#"} {
		if err := validateTopic(topic); err == nil {
			t.Fatalf("validateTopic(%q) succeeded, want error", topic)
		}
	}
	if err := validateTopic("zlan/rsds19y/state"); err != nil {
		t.Fatalf("validateTopic valid topic: %v", err)
	}
}
