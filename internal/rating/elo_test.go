package rating

import (
	"math"
	"testing"
)

func TestExpected(t *testing.T) {
	if got := Expected(1500, 1500); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("Expected(1500, 1500) = %v, want 0.5", got)
	}
	// 400 points is the scale's definition: ten to one.
	if got := Expected(1900, 1500); math.Abs(got-10.0/11.0) > 1e-9 {
		t.Errorf("Expected(1900, 1500) = %v, want 10/11", got)
	}
	if a, b := Expected(1600, 1400), Expected(1400, 1600); math.Abs(a+b-1) > 1e-9 {
		t.Errorf("expectations %v and %v do not sum to 1", a, b)
	}
}

func TestUpdateMovesBothWays(t *testing.T) {
	w, l := Update(1500, 1500, 32, 32)
	if w != 1516 || l != 1484 {
		t.Errorf("Update = %v, %v, want 1516, 1484", w, l)
	}

	// Beating someone far stronger moves a rating much further than beating
	// someone far weaker.
	upset, _ := Update(1500, 2000, 32, 32)
	expected, _ := Update(1500, 1000, 32, 32)
	if upset-1500 <= expected-1500 {
		t.Errorf("an upset gained %v and a formality %v", upset-1500, expected-1500)
	}
}

// K differs per player, so the two sides of a result are not equal and opposite.
func TestUpdateIsNotZeroSum(t *testing.T) {
	debutant, veteran := K(0), K(500)
	w, l := Update(1500, 1500, debutant, veteran)
	if w-1500 <= 1500-l {
		t.Errorf("the debutant gained %v, the veteran lost %v: want the newcomer to move more",
			w-1500, 1500-l)
	}
}
