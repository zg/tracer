package scene

import (
	"math"
	"math/rand"
	"testing"

	"github.com/zg/tracer/vec"
)

const eps = 1e-6

var mat = Lambertian{Albedo: vec.New(0.5, 0.5, 0.5)}

func TestRayAt(t *testing.T) {
	r := Ray{Origin: vec.New(1, 2, 3), Dir: vec.New(1, 0, 0)}
	if got, want := r.At(2), vec.New(3, 2, 3); got != want {
		t.Errorf("At(2) = %v, want %v", got, want)
	}
}

func TestSphereHit(t *testing.T) {
	s := Sphere{Center: vec.New(0, 0, -5), Radius: 1, Material: mat}
	tests := []struct {
		name   string
		ray    Ray
		wantOk bool
		wantT  float64
	}{
		{"head on", Ray{vec.Vec3{}, vec.New(0, 0, -1)}, true, 4},
		{"misses to the side", Ray{vec.Vec3{}, vec.New(0, 1, 0)}, false, 0},
		{"tangent misses", Ray{vec.New(0, 1.5, 0), vec.New(0, 0, -1)}, false, 0},
		{"pointing away", Ray{vec.Vec3{}, vec.New(0, 0, 1)}, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, ok := s.Hit(tt.ray, 1e-3, math.Inf(1))
			if ok != tt.wantOk {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOk)
			}
			if ok && math.Abs(h.T-tt.wantT) > eps {
				t.Errorf("t = %v, want %v", h.T, tt.wantT)
			}
		})
	}
}

// The recorded normal must always oppose the incoming ray, and FrontFace
// must say which side was struck. Dielectrics depend on this to know
// whether they are entering or leaving.
func TestSphereNormalOrientation(t *testing.T) {
	s := Sphere{Center: vec.New(0, 0, -5), Radius: 1, Material: mat}

	outside := Ray{vec.Vec3{}, vec.New(0, 0, -1)}
	h, ok := s.Hit(outside, 1e-3, math.Inf(1))
	if !ok {
		t.Fatal("expected a hit from outside")
	}
	if !h.FrontFace {
		t.Error("ray from outside should record FrontFace")
	}
	if h.Normal.Dot(outside.Dir) >= 0 {
		t.Error("normal must oppose the incoming ray")
	}

	inside := Ray{vec.New(0, 0, -5), vec.New(0, 0, -1)}
	h, ok = s.Hit(inside, 1e-3, math.Inf(1))
	if !ok {
		t.Fatal("expected a hit from inside")
	}
	if h.FrontFace {
		t.Error("ray from inside should not record FrontFace")
	}
	if h.Normal.Dot(inside.Dir) >= 0 {
		t.Error("normal must oppose the incoming ray even from inside")
	}
}

// A hit must lie on the surface: its distance from the centre is the radius.
func TestSphereHitPointOnSurface(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	s := Sphere{Center: vec.New(1, 2, 3), Radius: 2.5, Material: mat}
	hits := 0
	for range 2000 {
		r := Ray{
			Origin: vec.New(rng.NormFloat64()*5, rng.NormFloat64()*5, rng.NormFloat64()*5),
			Dir:    vec.New(rng.NormFloat64(), rng.NormFloat64(), rng.NormFloat64()),
		}
		h, ok := s.Hit(r, 1e-3, math.Inf(1))
		if !ok {
			continue
		}
		hits++
		if d := h.P.Sub(s.Center).Len(); math.Abs(d-s.Radius) > 1e-6 {
			t.Fatalf("hit point is %v from centre, want %v", d, s.Radius)
		}
		if math.Abs(h.Normal.Len()-1) > 1e-9 {
			t.Fatalf("normal is not unit length: %v", h.Normal.Len())
		}
	}
	if hits == 0 {
		t.Fatal("no hits generated; test proved nothing")
	}
}

func TestSphereRespectsTRange(t *testing.T) {
	s := Sphere{Center: vec.New(0, 0, -5), Radius: 1, Material: mat}
	r := Ray{vec.Vec3{}, vec.New(0, 0, -1)}
	if _, ok := s.Hit(r, 1e-3, 3); ok {
		t.Error("hit at t=4 should be rejected when tMax is 3")
	}
	// With tMin past the near root the far root is still a legitimate hit:
	// the ray leaves through the back of the sphere at t=6.
	h, ok := s.Hit(r, 5, math.Inf(1))
	if !ok {
		t.Fatal("far root at t=6 should still be found when tMin is 5")
	}
	if math.Abs(h.T-6) > eps {
		t.Errorf("t = %v, want 6 (the far root)", h.T)
	}
	if h.FrontFace {
		t.Error("hitting the far wall from inside should not be a front face")
	}
	if _, ok := s.Hit(r, 7, math.Inf(1)); ok {
		t.Error("both roots are before tMin=7, so there should be no hit")
	}
}

func TestPlaneHit(t *testing.T) {
	p := Plane{Point: vec.New(0, -1, 0), Normal: vec.New(0, 1, 0), Material: mat}
	h, ok := p.Hit(Ray{vec.Vec3{}, vec.New(0, -1, 0)}, 1e-3, math.Inf(1))
	if !ok {
		t.Fatal("expected a hit")
	}
	if math.Abs(h.T-1) > eps {
		t.Errorf("t = %v, want 1", h.T)
	}
	// Parallel rays never meet the plane.
	if _, ok := p.Hit(Ray{vec.Vec3{}, vec.New(1, 0, 0)}, 1e-3, math.Inf(1)); ok {
		t.Error("parallel ray should not hit")
	}
}

func TestTriangleHit(t *testing.T) {
	tri := Triangle{
		A: vec.New(0, 0, -2), B: vec.New(1, 0, -2), C: vec.New(0, 1, -2),
		Material: mat,
	}
	// Through the interior.
	if h, ok := tri.Hit(Ray{vec.New(0.25, 0.25, 0), vec.New(0, 0, -1)}, 1e-3, math.Inf(1)); !ok {
		t.Error("expected a hit inside the triangle")
	} else if math.Abs(h.T-2) > eps {
		t.Errorf("t = %v, want 2", h.T)
	}
	// Outside the triangle but on its plane.
	if _, ok := tri.Hit(Ray{vec.New(0.9, 0.9, 0), vec.New(0, 0, -1)}, 1e-3, math.Inf(1)); ok {
		t.Error("point outside the triangle should not hit")
	}
	// Parallel to the plane.
	if _, ok := tri.Hit(Ray{vec.New(0.25, 0.25, 0), vec.New(1, 0, 0)}, 1e-3, math.Inf(1)); ok {
		t.Error("parallel ray should not hit")
	}
}

func TestListReturnsNearest(t *testing.T) {
	near := Sphere{Center: vec.New(0, 0, -2), Radius: 0.5, Material: mat}
	far := Sphere{Center: vec.New(0, 0, -10), Radius: 0.5, Material: mat}
	l := List{far, near} // deliberately out of order
	h, ok := l.Hit(Ray{vec.Vec3{}, vec.New(0, 0, -1)}, 1e-3, math.Inf(1))
	if !ok {
		t.Fatal("expected a hit")
	}
	if math.Abs(h.T-1.5) > eps {
		t.Errorf("t = %v, want 1.5 (the nearer sphere)", h.T)
	}
}

func TestEmptyListMisses(t *testing.T) {
	if _, ok := (List{}).Hit(Ray{vec.Vec3{}, vec.New(0, 0, -1)}, 1e-3, math.Inf(1)); ok {
		t.Error("empty list cannot be hit")
	}
}

func TestAABBHit(t *testing.T) {
	box := AABB{Min: vec.New(-1, -1, -1), Max: vec.New(1, 1, 1)}
	if !box.Hit(Ray{vec.New(0, 0, -5), vec.New(0, 0, 1)}, 1e-3, math.Inf(1)) {
		t.Error("ray through the box should hit")
	}
	if box.Hit(Ray{vec.New(5, 5, -5), vec.New(0, 0, 1)}, 1e-3, math.Inf(1)) {
		t.Error("ray missing the box should not hit")
	}
}

// A ray exactly parallel to a slab, lying on its boundary, produces NaN in
// the naive slab test. It must not crash or silently report a miss.
func TestAABBParallelRayOnBoundary(t *testing.T) {
	box := AABB{Min: vec.New(-1, -1, -1), Max: vec.New(1, 1, 1)}
	// Origin y is exactly on the box's max plane, direction has zero y.
	r := Ray{Origin: vec.New(0, 1, -5), Dir: vec.New(0, 0, 1)}
	_ = box.Hit(r, 1e-3, math.Inf(1)) // must simply not panic or hang
}

// The BVH is an acceleration structure: it must return exactly what the
// brute-force list returns, for every ray.
func TestBVHMatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	var surfaces []Surface
	for range 200 {
		surfaces = append(surfaces, Sphere{
			Center:   vec.New(rng.Float64()*20-10, rng.Float64()*20-10, rng.Float64()*20-10),
			Radius:   rng.Float64()*0.8 + 0.2,
			Material: mat,
		})
	}
	brute := make(List, len(surfaces))
	copy(brute, surfaces)

	bvh := NewBVH(surfaces) // reorders its argument, hence the copy above

	tested := 0
	for range 5000 {
		r := Ray{
			Origin: vec.New(rng.NormFloat64()*8, rng.NormFloat64()*8, rng.NormFloat64()*8),
			Dir:    vec.New(rng.NormFloat64(), rng.NormFloat64(), rng.NormFloat64()),
		}
		wantHit, wantOk := brute.Hit(r, 1e-3, math.Inf(1))
		gotHit, gotOk := bvh.Hit(r, 1e-3, math.Inf(1))
		if wantOk != gotOk {
			t.Fatalf("BVH ok = %v, brute force ok = %v", gotOk, wantOk)
		}
		if wantOk {
			tested++
			if math.Abs(gotHit.T-wantHit.T) > 1e-9 {
				t.Fatalf("BVH t = %v, brute force t = %v", gotHit.T, wantHit.T)
			}
		}
	}
	if tested < 100 {
		t.Fatalf("only %d hits exercised; test proved little", tested)
	}
}

func TestBVHDegenerateSizes(t *testing.T) {
	for _, n := range []int{0, 1, 2, 3} {
		var surfaces []Surface
		for i := range n {
			surfaces = append(surfaces, Sphere{Center: vec.New(float64(i), 0, -5), Radius: 0.4, Material: mat})
		}
		s := NewBVH(surfaces)
		if s == nil {
			t.Fatalf("n=%d produced a nil surface", n)
		}
		_, _ = s.Hit(Ray{vec.Vec3{}, vec.New(0, 0, -1)}, 1e-3, math.Inf(1))
	}
}

func TestLambertianScatterStaysAboveSurface(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	m := Lambertian{Albedo: vec.New(0.5, 0.5, 0.5)}
	h := Hit{P: vec.Vec3{}, Normal: vec.New(0, 1, 0), FrontFace: true}
	for range 5000 {
		out, att, ok := m.Scatter(Ray{}, h, rng)
		if !ok {
			t.Fatal("lambertian must always scatter")
		}
		if math.IsNaN(out.Dir.X) || math.IsNaN(out.Dir.Y) || math.IsNaN(out.Dir.Z) {
			t.Fatal("scatter produced a NaN direction")
		}
		if att != m.Albedo {
			t.Fatalf("attenuation = %v, want %v", att, m.Albedo)
		}
	}
}

// Fuzz can push a metal's reflection below the surface; those rays must be
// absorbed rather than escaping through the object.
func TestMetalAbsorbsRaysBelowSurface(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	m := Metal{Albedo: vec.New(0.8, 0.8, 0.8), Fuzz: 1}
	h := Hit{P: vec.Vec3{}, Normal: vec.New(0, 1, 0), FrontFace: true}
	// Grazing incidence: the reflection comes off nearly tangent to the
	// surface, which is the only regime where fuzz can push it below.
	// A head-on ray reflects nearly straight up and fuzz of 1 can never
	// flip it, so that case would prove nothing.
	in := Ray{Origin: vec.New(-1, 0.01, 0), Dir: vec.New(1, -0.01, 0)}

	absorbed := 0
	for range 5000 {
		out, _, ok := m.Scatter(in, h, rng)
		if !ok {
			absorbed++
			continue
		}
		if out.Dir.Dot(h.Normal) <= 0 {
			t.Fatal("scattered ray points into the surface")
		}
	}
	if absorbed == 0 {
		t.Error("with fuzz 1 at a grazing angle, some rays should be absorbed")
	}
}

func TestMetalFuzzIsClamped(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	// Fuzz far outside [0,1] must not produce wild directions.
	m := Metal{Albedo: vec.New(1, 1, 1), Fuzz: 100}
	h := Hit{P: vec.Vec3{}, Normal: vec.New(0, 1, 0), FrontFace: true}
	in := Ray{Origin: vec.New(0, 1, 0), Dir: vec.New(0, -1, 0)}
	for range 1000 {
		if out, _, ok := m.Scatter(in, h, rng); ok {
			if out.Dir.Len() > 3 {
				t.Fatalf("fuzz was not clamped: direction length %v", out.Dir.Len())
			}
		}
	}
}

// Beyond the critical angle a dielectric must reflect rather than refract.
func TestDielectricTotalInternalReflection(t *testing.T) {
	rng := rand.New(rand.NewSource(6))
	m := Dielectric{RefIdx: 1.5}
	// Inside the glass (FrontFace false) at a grazing angle.
	h := Hit{P: vec.Vec3{}, Normal: vec.New(0, 1, 0), FrontFace: false}
	in := Ray{Dir: vec.New(1, -0.05, 0).Unit()}
	out, att, ok := m.Scatter(in, h, rng)
	if !ok {
		t.Fatal("dielectric must always scatter")
	}
	if att != vec.New(1, 1, 1) {
		t.Errorf("glass should not attenuate: %v", att)
	}
	// Reflected, so it must stay on the incoming side of the surface.
	if out.Dir.Dot(h.Normal) <= 0 {
		t.Error("total internal reflection should send the ray back")
	}
}

func TestDiffuseLightEmitsAndDoesNotScatter(t *testing.T) {
	m := DiffuseLight{Colour: vec.New(4, 4, 4)}
	if _, _, ok := m.Scatter(Ray{}, Hit{}, rand.New(rand.NewSource(1))); ok {
		t.Error("a light should not scatter")
	}
	if m.Emitted() != vec.New(4, 4, 4) {
		t.Errorf("Emitted = %v", m.Emitted())
	}
}

func TestCameraRayHitsCentre(t *testing.T) {
	cam := NewCamera(CameraOptions{
		LookFrom: vec.New(0, 0, 0), LookAt: vec.New(0, 0, -1),
		VFOVDegrees: 90, AspectRatio: 1,
	})
	rng := rand.New(rand.NewSource(1))
	// The centre of the film must look straight down -Z.
	r := cam.Ray(0.5, 0.5, rng)
	d := r.Dir.Unit()
	if math.Abs(d.X) > 1e-9 || math.Abs(d.Y) > 1e-9 || math.Abs(d.Z+1) > 1e-9 {
		t.Errorf("centre ray direction = %v, want (0,0,-1)", d)
	}
}

func TestCameraDefaults(t *testing.T) {
	// A zero-value options struct must still produce a usable camera
	// rather than NaNs from a zero focus distance or aspect ratio.
	cam := NewCamera(CameraOptions{})
	r := cam.Ray(0.5, 0.5, rand.New(rand.NewSource(1)))
	if math.IsNaN(r.Dir.X) || math.IsNaN(r.Dir.Y) || math.IsNaN(r.Dir.Z) {
		t.Fatalf("default camera produced NaN direction: %v", r.Dir)
	}
}

func TestCameraApertureJittersOrigin(t *testing.T) {
	cam := NewCamera(CameraOptions{
		LookFrom: vec.New(0, 0, 0), LookAt: vec.New(0, 0, -1),
		Aperture: 2, FocusDistance: 1,
	})
	rng := rand.New(rand.NewSource(1))
	seen := map[vec.Vec3]bool{}
	for range 50 {
		seen[cam.Ray(0.5, 0.5, rng).Origin] = true
	}
	if len(seen) < 2 {
		t.Error("a non-zero aperture should jitter the ray origin across the lens")
	}
}
