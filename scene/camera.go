package scene

import (
	"math"
	"math/rand"

	"github.com/zg/tracer/vec"
)

// Camera generates rays through a virtual film plane.
type Camera struct {
	origin     vec.Vec3
	lowerLeft  vec.Vec3
	horizontal vec.Vec3
	vertical   vec.Vec3
	u, v       vec.Vec3
	lensRadius float64
}

// CameraOptions configures a camera.
type CameraOptions struct {
	LookFrom, LookAt vec.Vec3
	Up               vec.Vec3
	VFOVDegrees      float64 // vertical field of view
	AspectRatio      float64
	Aperture         float64 // 0 for a pinhole camera (everything in focus)
	FocusDistance    float64 // 0 means focus on LookAt
}

// NewCamera builds a camera from opts, filling in sensible defaults.
func NewCamera(opts CameraOptions) *Camera {
	if opts.Up == (vec.Vec3{}) {
		opts.Up = vec.New(0, 1, 0)
	}
	if opts.VFOVDegrees <= 0 {
		opts.VFOVDegrees = 40
	}
	if opts.AspectRatio <= 0 {
		opts.AspectRatio = 16.0 / 9.0
	}
	focus := opts.FocusDistance
	if focus <= 0 {
		focus = opts.LookFrom.Sub(opts.LookAt).Len()
	}
	if focus <= 0 {
		focus = 1
	}

	theta := opts.VFOVDegrees * math.Pi / 180
	h := math.Tan(theta / 2)
	viewportHeight := 2 * h
	viewportWidth := opts.AspectRatio * viewportHeight

	w := opts.LookFrom.Sub(opts.LookAt).Unit()
	u := opts.Up.Cross(w).Unit()
	v := w.Cross(u)

	c := &Camera{
		origin:     opts.LookFrom,
		horizontal: u.Scale(viewportWidth * focus),
		vertical:   v.Scale(viewportHeight * focus),
		u:          u,
		v:          v,
		lensRadius: opts.Aperture / 2,
	}
	c.lowerLeft = c.origin.
		Sub(c.horizontal.Scale(0.5)).
		Sub(c.vertical.Scale(0.5)).
		Sub(w.Scale(focus))
	return c
}

// Ray returns the ray through normalised film coordinates (s,t), both in
// [0,1]. With a non-zero aperture the origin is jittered across the lens,
// which is what produces depth of field.
func (c *Camera) Ray(s, t float64, rng *rand.Rand) Ray {
	origin := c.origin
	if c.lensRadius > 0 {
		rd := randomInUnitDisk(rng).Scale(c.lensRadius)
		origin = origin.Add(c.u.Scale(rd.X)).Add(c.v.Scale(rd.Y))
	}
	target := c.lowerLeft.Add(c.horizontal.Scale(s)).Add(c.vertical.Scale(t))
	return Ray{Origin: origin, Dir: target.Sub(origin)}
}

func randomInUnitDisk(rng *rand.Rand) vec.Vec3 {
	for {
		p := vec.New(rng.Float64()*2-1, rng.Float64()*2-1, 0)
		if p.LenSq() <= 1 {
			return p
		}
	}
}
