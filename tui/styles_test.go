package tui

import (
	"testing"
)

func TestStyleColors_NonNil(t *testing.T) {
	tests := []struct {
		name string
		c    interface{}
	}{
		{"accentColor", accentColor},
		{"dimColor", dimColor},
		{"greenColor", greenColor},
		{"yellowColor", yellowColor},
		{"redColor", redColor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.c == nil {
				t.Errorf("%s is nil", tt.name)
			}
		})
	}
}

func TestBorderStyles_Render(t *testing.T) {
	bs := borderStyle.Render("test")
	if bs == "" {
		t.Error("borderStyle.Render('test') returned empty")
	}
	fbs := focusedBorderStyle.Render("test")
	if fbs == "" {
		t.Error("focusedBorderStyle.Render('test') returned empty")
	}
}

func TestMax(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{1, 2, 2},
		{5, 3, 5},
		{0, 0, 0},
		{-1, 1, 1},
		{-5, -3, -3},
	}
	for _, tt := range tests {
		got := max(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("max(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
