// Package scene defines the geometry, materials and camera the renderer
// traces rays against.
package scene

import (
	"math"
	"math/rand"

	"github.com/zg/tracer/vec"
)

// Ray is a parametric line: Origin + t*Dir for t >= 0.
type Ray struct {
	Origin, Dir vec.Vec3
}

// At returns the point t units along the ray.
func (r Ray) At(t float64) vec.Vec3 { return r.Origin.Add(r.Dir.Scale(t)) }

// Hit records an intersection between a ray and a surface.
type Hit struct {
	P         vec.Vec3 // point of intersection
	Normal    vec.Vec3 // unit surface normal, always facing the incoming ray
	T         float64  // ray parameter at P
	Material  Material
	FrontFace bool // whether the ray struck the outside of the surface
}

// setFaceNormal orients the stored normal against the incoming ray, so
// shading never has to worry which side it is on. FrontFace is kept because
// dielectrics need to know whether they are entering or leaving the glass.
func (h *Hit) setFaceNormal(r Ray, outward vec.Vec3) {
	h.FrontFace = r.Dir.Dot(outward) < 0
	if h.FrontFace {
		h.Normal = outward
	} else {
		h.Normal = outward.Neg()
	}
}

// Surface is anything a ray can hit.
type Surface interface {
	// Hit reports the nearest intersection with t in (tMin, tMax).
	Hit(r Ray, tMin, tMax float64) (Hit, bool)
	// Bounds returns an axis-aligned bounding box enclosing the surface.
	Bounds() AABB
}

// Sphere is a sphere of the given radius centred at Center.
type Sphere struct {
	Center   vec.Vec3
	Radius   float64
	Material Material
}

func (s Sphere) Hit(r Ray, tMin, tMax float64) (Hit, bool) {
	// Solve |O + tD - C|^2 = R^2 for t. Using half-b removes a factor of
	// two from the quadratic formula and a little floating point noise.
	oc := r.Origin.Sub(s.Center)
	a := r.Dir.LenSq()
	halfB := oc.Dot(r.Dir)
	c := oc.LenSq() - s.Radius*s.Radius

	disc := halfB*halfB - a*c
	if disc < 0 {
		return Hit{}, false
	}
	sqrtD := math.Sqrt(disc)

	// Prefer the nearer root, falling back to the farther one when the
	// nearer is behind the ray or beyond the search window.
	root := (-halfB - sqrtD) / a
	if root <= tMin || root >= tMax {
		root = (-halfB + sqrtD) / a
		if root <= tMin || root >= tMax {
			return Hit{}, false
		}
	}

	h := Hit{P: r.At(root), T: root, Material: s.Material}
	h.setFaceNormal(r, h.P.Sub(s.Center).Scale(1/s.Radius))
	return h, true
}

func (s Sphere) Bounds() AABB {
	rv := vec.New(s.Radius, s.Radius, s.Radius)
	return AABB{Min: s.Center.Sub(rv), Max: s.Center.Add(rv)}
}

// Plane is an infinite plane through Point with the given unit Normal.
type Plane struct {
	Point    vec.Vec3
	Normal   vec.Vec3
	Material Material
}

func (p Plane) Hit(r Ray, tMin, tMax float64) (Hit, bool) {
	denom := p.Normal.Dot(r.Dir)
	if math.Abs(denom) < 1e-9 {
		return Hit{}, false // ray is parallel to the plane
	}
	t := p.Point.Sub(r.Origin).Dot(p.Normal) / denom
	if t <= tMin || t >= tMax {
		return Hit{}, false
	}
	h := Hit{P: r.At(t), T: t, Material: p.Material}
	h.setFaceNormal(r, p.Normal)
	return h, true
}

// Bounds returns a deliberately huge box: an infinite plane has no finite
// extent, so it cannot participate usefully in BVH partitioning.
func (p Plane) Bounds() AABB {
	const big = 1e9
	return AABB{Min: vec.New(-big, -big, -big), Max: vec.New(big, big, big)}
}

// Triangle is a single triangle, hit-tested with the Möller–Trumbore
// algorithm.
type Triangle struct {
	A, B, C  vec.Vec3
	Material Material
}

func (tri Triangle) Hit(r Ray, tMin, tMax float64) (Hit, bool) {
	const eps = 1e-9
	edge1 := tri.B.Sub(tri.A)
	edge2 := tri.C.Sub(tri.A)
	h := r.Dir.Cross(edge2)
	a := edge1.Dot(h)
	if math.Abs(a) < eps {
		return Hit{}, false // parallel to the triangle plane
	}

	f := 1 / a
	s := r.Origin.Sub(tri.A)
	u := f * s.Dot(h)
	if u < 0 || u > 1 {
		return Hit{}, false
	}
	q := s.Cross(edge1)
	v := f * r.Dir.Dot(q)
	if v < 0 || u+v > 1 {
		return Hit{}, false
	}

	t := f * edge2.Dot(q)
	if t <= tMin || t >= tMax {
		return Hit{}, false
	}

	hit := Hit{P: r.At(t), T: t, Material: tri.Material}
	hit.setFaceNormal(r, edge1.Cross(edge2).Unit())
	return hit, true
}

func (tri Triangle) Bounds() AABB {
	min := vec.New(
		math.Min(tri.A.X, math.Min(tri.B.X, tri.C.X)),
		math.Min(tri.A.Y, math.Min(tri.B.Y, tri.C.Y)),
		math.Min(tri.A.Z, math.Min(tri.B.Z, tri.C.Z)),
	)
	max := vec.New(
		math.Max(tri.A.X, math.Max(tri.B.X, tri.C.X)),
		math.Max(tri.A.Y, math.Max(tri.B.Y, tri.C.Y)),
		math.Max(tri.A.Z, math.Max(tri.B.Z, tri.C.Z)),
	)
	// A triangle lying exactly in an axis plane would have zero thickness,
	// giving a degenerate box that slab tests can miss. Pad it.
	return AABB{Min: min, Max: max}.Padded(1e-6)
}

// List is a flat collection of surfaces, tested one by one. It is the
// fallback when a BVH is not worth building.
type List []Surface

func (l List) Hit(r Ray, tMin, tMax float64) (Hit, bool) {
	var best Hit
	found := false
	closest := tMax
	for _, s := range l {
		if h, ok := s.Hit(r, tMin, closest); ok {
			found, closest, best = true, h.T, h
		}
	}
	return best, found
}

func (l List) Bounds() AABB {
	if len(l) == 0 {
		return AABB{}
	}
	b := l[0].Bounds()
	for _, s := range l[1:] {
		b = b.Union(s.Bounds())
	}
	return b
}

// randomUnitVector returns a uniformly distributed direction on the unit
// sphere, used for diffuse scattering.
func randomUnitVector(rng *rand.Rand) vec.Vec3 {
	// Rejection sampling inside the unit ball, then normalise. Sampling
	// each component uniformly and normalising without rejection would
	// bias directions toward the cube's corners.
	for {
		p := vec.New(
			rng.Float64()*2-1,
			rng.Float64()*2-1,
			rng.Float64()*2-1,
		)
		if l := p.LenSq(); l > 0 && l <= 1 {
			return p.Unit()
		}
	}
}
