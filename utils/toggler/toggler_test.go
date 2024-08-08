package toggler

import (
	"testing"
)

func TestSwtich(t *testing.T) {
	var s Switch

	// go func() {
	// 	for i := 0; i < 10; i++ {
	// 		s.On()
	// 		time.Sleep(1)
	// 		s.Off()
	// 		time.Sleep(1)
	// 	}
	// }()

	s.On()
	if s.State() != On {
		t.Error(s.State())
	}

}
