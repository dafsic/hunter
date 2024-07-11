package utils

import (
	"testing"
)

func TestSwtich(t *testing.T) {
	var s Switcher

	// go func() {
	// 	for i := 0; i < 10; i++ {
	// 		s.On()
	// 		time.Sleep(1)
	// 		s.Off()
	// 		time.Sleep(1)
	// 	}
	// }()

	s.On()
	if s.State() != Switcher(1) {
		t.Error(s.State())
	}

}
