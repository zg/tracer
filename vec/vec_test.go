package vec

import (
	"math"
	"math/rand"
	"testing"
)

const eps = 1e-9

func close(a, b float64) bool { return math.Abs(a-b) < eps }
func vclose(a, b Vec3) bool   { return close(a.X, b.X) && close(a.Y, b.Y) && close(a.Z, b.Z) }

func TestArithmetic(t *testing.T) {
	a, b := New(1, 2, 3), New(4, 5, 6)
	if got, want := a.Add(b), New(5, 7, 9); !vclose(got, want) {
		t.Errorf("Add = %v, want %v", got, want)
	}
	if got, want := a.Sub(b), New(-3, -3, -3); !vclose(got, want) {
		t.Errorf("Sub = %v, want %v", got, want)
	}
	if got, want := a.Mul(b), New(4, 10, 18); !vclose(got, want) {
		t.Errorf("Mul = %v, want %v", got, want)
	}
	if got, want := a.Scale(2), New(2, 4, 6); !vclose(got, want) {
		t.Errorf("Scale = %v, want %v", got, want)
	}
	if got, want := a.Dot(b), 32.0; !close(got, want) {
		t.Errorf("Dot = %v, want %v", got, want)
	}
}

func TestCrossIsPerpendicular(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for range 500 {
		a := New(rng.NormFloat64(), rng.NormFloat64(), rng.NormFloat64())
		b := New(rng.NormFloat64(), rng.NormFloat64(), rng.NormFloat64())
		c := a.Cross(b)
		if !close(c.Dot(a), 0) || !close(c.Dot(b), 0) {
			t.Fatalf("cross of %v and %v is not perpendicular: %v", a, b, c)
		}
	}
}

func TestCrossAntiCommutative(t *testing.T) {
	a, b := New(1, 2, 3), New(4, 5, 6)
	if !vclose(a.Cross(b), b.Cross(a).Neg()) {
		t.Error("a×b must equal -(b×a)")
	}
}

func TestUnit(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	for range 500 {
		v := New(rng.NormFloat64(), rng.NormFloat64(), rng.NormFloat64())
		if v.LenSq() == 0 {
			continue
		}
		if got := v.Unit().Len(); !close(got, 1) {
			t.Fatalf("unit length = %v, want 1", got)
		}
	}
}

// A zero vector has no direction; normalising must not produce NaNs, which
// would propagate silently through an entire render.
func TestUnitZeroVector(t *testing.T) {
	got := Vec3{}.Unit()
	if math.IsNaN(got.X) || math.IsNaN(got.Y) || math.IsNaN(got.Z) {
		t.Fatalf("zero vector normalised to NaN: %v", got)
	}
	if got != (Vec3{}) {
		t.Errorf("= %v, want zero vector", got)
	}
}

func TestNearZero(t *testing.T) {
	if !(Vec3{1e-12, 1e-12, 1e-12}).NearZero() {
		t.Error("tiny vector should be near zero")
	}
	if (Vec3{1, 0, 0}).NearZero() {
		t.Error("unit x should not be near zero")
	}
}

// Reflection must preserve length and mirror the normal component.
func TestReflect(t *testing.T) {
	// Straight down onto a floor bounces straight up.
	got := Reflect(New(0, -1, 0), New(0, 1, 0))
	if !vclose(got, New(0, 1, 0)) {
		t.Errorf("Reflect = %v, want (0,1,0)", got)
	}

	rng := rand.New(rand.NewSource(3))
	for range 500 {
		v := New(rng.NormFloat64(), rng.NormFloat64(), rng.NormFloat64())
		n := New(rng.NormFloat64(), rng.NormFloat64(), rng.NormFloat64()).Unit()
		r := Reflect(v, n)
		if !close(r.Len(), v.Len()) {
			t.Fatalf("reflection changed length: %v -> %v", v.Len(), r.Len())
		}
		// The component along n flips sign; the tangential part is kept.
		if !close(r.Dot(n), -v.Dot(n)) {
			t.Fatalf("normal component not mirrored")
		}
	}
}

// Refracting through a ratio of 1 (no index change) must leave the ray
// unbent.
func TestRefractIdentity(t *testing.T) {
	v := New(1, -1, 0).Unit()
	n := New(0, 1, 0)
	if got := Refract(v, n, 1.0); !vclose(got, v) {
		t.Errorf("Refract with ratio 1 = %v, want %v", got, v)
	}
}

func TestRefractBendsTowardNormal(t *testing.T) {
	// Entering a denser medium (ratio < 1) bends the ray toward the normal,
	// so the angle from the normal shrinks.
	v := New(1, -1, 0).Unit()
	n := New(0, 1, 0)
	out := Refract(v, n, 0.5)

	inAngle := math.Acos(math.Min(v.Neg().Dot(n), 1))
	outAngle := math.Acos(math.Min(out.Neg().Dot(n), 1))
	if outAngle >= inAngle {
		t.Errorf("ray did not bend toward the normal: %.4f -> %.4f", inAngle, outAngle)
	}
}

func TestLerp(t *testing.T) {
	a, b := New(0, 0, 0), New(10, 20, 30)
	if got := Lerp(a, b, 0); !vclose(got, a) {
		t.Errorf("t=0 gave %v, want %v", got, a)
	}
	if got := Lerp(a, b, 1); !vclose(got, b) {
		t.Errorf("t=1 gave %v, want %v", got, b)
	}
	if got := Lerp(a, b, 0.5); !vclose(got, New(5, 10, 15)) {
		t.Errorf("t=0.5 gave %v", got)
	}
}
