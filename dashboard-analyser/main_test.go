package main

import (
	"testing"
)

func Test_findMaxInt(t *testing.T) {
	numbers := []int{0, -1, -12, 90.0, 120}
	max := findMaxInt(numbers)
	if max != 120 {
		t.Error("Expected 120, got: ", max)
	}

}

func Test_filterNumbersFromTextDetections(t *testing.T) {

}
