package pm25

import (
	"fmt"
	"testing"
)

var err error

func Test_GrabData(t *testing.T) {
	fmt.Println("\nTesting GrabData()...")

	s, err := GetUSEmbassyStation("Shanghai")
	if err != nil {
		fmt.Println(err)
		t.Error(err)
	}

	err = s.GrabData()
	if err != nil {
		fmt.Println(err)
		t.Error(err)
	}
}
