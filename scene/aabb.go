package scene

import (
	"math"
	"sort"

	"github.com/zg/tracer/vec"
)

// AABB is an axis-aligned bounding box.
type AABB struct{ Min, Max vec.Vec3 }

// Union returns the smallest box containing both a and b.
func (a AABB) Union(b AABB) AABB {
	return AABB{
		Min: vec.New(math.Min(a.Min.X, b.Min.X), math.Min(a.Min.Y, b.Min.Y), math.Min(a.Min.Z, b.Min.Z)),
		Max: vec.New(math.Max(a.Max.X, b.Max.X), math.Max(a.Max.Y, b.Max.Y), math.Max(a.Max.Z, b.Max.Z)),
	}
}

// Padded grows the box by d on every axis, so a flat box (a triangle in an
// axis plane, say) still has non-zero thickness for the slab test.
func (a AABB) Padded(d float64) AABB {
	p := vec.New(d, d, d)
	return AABB{Min: a.Min.Sub(p), Max: a.Max.Add(p)}
}

// Hit reports whether r passes through the box within (tMin, tMax), using
// the slab method.
func (a AABB) Hit(r Ray, tMin, tMax float64) bool {
	for axis := range 3 {
		o, d := component(r.Origin, axis), component(r.Dir, axis)
		// A zero direction component means the ray is parallel to this
		// slab. 1/0 gives ±Inf, and the min/max below then correctly
		// either keep or reject the interval — except when the origin is
		// exactly on a boundary, where it yields NaN. Guard that case.
		invD := 1 / d
		t0 := (component(a.Min, axis) - o) * invD
		t1 := (component(a.Max, axis) - o) * invD
		if invD < 0 {
			t0, t1 = t1, t0
		}
		if math.IsNaN(t0) || math.IsNaN(t1) {
			continue
		}
		if t0 > tMin {
			tMin = t0
		}
		if t1 < tMax {
			tMax = t1
		}
		if tMax <= tMin {
			return false
		}
	}
	return true
}

func component(v vec.Vec3, axis int) float64 {
	switch axis {
	case 0:
		return v.X
	case 1:
		return v.Y
	default:
		return v.Z
	}
}

// BVH is a bounding volume hierarchy: a binary tree of bounding boxes that
// lets a ray skip whole groups of surfaces at once. Without it, every ray
// tests every surface, and render time grows linearly with scene size.
type BVH struct {
	left, right Surface
	box         AABB
}

// NewBVH builds a hierarchy over surfaces. The slice is reordered.
func NewBVH(surfaces []Surface) Surface {
	if len(surfaces) == 0 {
		return List{}
	}
	if len(surfaces) <= 2 {
		return List(surfaces)
	}

	// Split along whichever axis the centroids are most spread over: a
	// fixed or random axis produces badly unbalanced trees for scenes laid
	// out along one direction, such as a row of spheres on a plane.
	axis := widestAxis(surfaces)
	sort.Slice(surfaces, func(i, j int) bool {
		return centroid(surfaces[i].Bounds(), axis) < centroid(surfaces[j].Bounds(), axis)
	})

	mid := len(surfaces) / 2
	left := NewBVH(surfaces[:mid])
	right := NewBVH(surfaces[mid:])
	return &BVH{left: left, right: right, box: left.Bounds().Union(right.Bounds())}
}

func (b *BVH) Bounds() AABB { return b.box }

func (b *BVH) Hit(r Ray, tMin, tMax float64) (Hit, bool) {
	if !b.box.Hit(r, tMin, tMax) {
		return Hit{}, false
	}
	hitLeft, okLeft := b.left.Hit(r, tMin, tMax)
	if okLeft {
		// Shrink the search window so the right subtree can only win by
		// being nearer.
		tMax = hitLeft.T
	}
	if hitRight, okRight := b.right.Hit(r, tMin, tMax); okRight {
		return hitRight, true
	}
	return hitLeft, okLeft
}

func widestAxis(surfaces []Surface) int {
	b := surfaces[0].Bounds()
	for _, s := range surfaces[1:] {
		b = b.Union(s.Bounds())
	}
	extent := b.Max.Sub(b.Min)
	switch {
	case extent.X >= extent.Y && extent.X >= extent.Z:
		return 0
	case extent.Y >= extent.Z:
		return 1
	default:
		return 2
	}
}

func centroid(b AABB, axis int) float64 {
	return (component(b.Min, axis) + component(b.Max, axis)) / 2
}
