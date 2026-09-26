package scene

import (
	"math"
	"math/rand"

	"github.com/zg/tracer/vec"
)

// Material decides how a surface scatters incoming light.
type Material interface {
	// Scatter returns the outgoing ray and the attenuation applied to it.
	// ok is false when the ray is absorbed.
	Scatter(in Ray, h Hit, rng *rand.Rand) (out Ray, attenuation vec.Vec3, ok bool)
	// Emitted returns light the surface emits on its own.
	Emitted() vec.Vec3
}

// Lambertian is an ideal matte surface: it scatters uniformly about the
// normal.
type Lambertian struct{ Albedo vec.Vec3 }

func (m Lambertian) Scatter(_ Ray, h Hit, rng *rand.Rand) (Ray, vec.Vec3, bool) {
	dir := h.Normal.Add(randomUnitVector(rng))
	// The random vector can exactly cancel the normal, leaving a zero
	// direction that would normalise to NaN and blacken the pixel.
	if dir.NearZero() {
		dir = h.Normal
	}
	return Ray{Origin: h.P, Dir: dir}, m.Albedo, true
}

func (m Lambertian) Emitted() vec.Vec3 { return vec.Vec3{} }

// Metal is a reflective surface. Fuzz perturbs the reflected direction,
// turning a mirror into a brushed finish.
type Metal struct {
	Albedo vec.Vec3
	Fuzz   float64
}

func (m Metal) Scatter(in Ray, h Hit, rng *rand.Rand) (Ray, vec.Vec3, bool) {
	fuzz := math.Min(math.Max(m.Fuzz, 0), 1)
	reflected := vec.Reflect(in.Dir.Unit(), h.Normal).Add(randomUnitVector(rng).Scale(fuzz))
	// Fuzz can push the ray below the surface; absorb it rather than
	// letting it escape through the object.
	if reflected.Dot(h.Normal) <= 0 {
		return Ray{}, vec.Vec3{}, false
	}
	return Ray{Origin: h.P, Dir: reflected}, m.Albedo, true
}

func (m Metal) Emitted() vec.Vec3 { return vec.Vec3{} }

// Dielectric is a transparent material such as glass or water, described by
// its refractive index.
type Dielectric struct{ RefIdx float64 }

func (m Dielectric) Scatter(in Ray, h Hit, rng *rand.Rand) (Ray, vec.Vec3, bool) {
	ratio := m.RefIdx
	if h.FrontFace {
		ratio = 1 / m.RefIdx // entering the denser medium
	}

	unit := in.Dir.Unit()
	cosTheta := math.Min(unit.Neg().Dot(h.Normal), 1)
	sinTheta := math.Sqrt(1 - cosTheta*cosTheta)

	// Past the critical angle there is no transmitted ray at all, so the
	// surface behaves as a perfect mirror. Schlick's approximation adds the
	// angle-dependent partial reflection below that angle.
	mustReflect := ratio*sinTheta > 1
	if mustReflect || schlick(cosTheta, ratio) > rng.Float64() {
		return Ray{Origin: h.P, Dir: vec.Reflect(unit, h.Normal)}, vec.New(1, 1, 1), true
	}
	return Ray{Origin: h.P, Dir: vec.Refract(unit, h.Normal, ratio)}, vec.New(1, 1, 1), true
}

func (m Dielectric) Emitted() vec.Vec3 { return vec.Vec3{} }

// schlick approximates the Fresnel reflectance at a given angle.
func schlick(cosine, refIdx float64) float64 {
	r0 := (1 - refIdx) / (1 + refIdx)
	r0 *= r0
	return r0 + (1-r0)*math.Pow(1-cosine, 5)
}

// DiffuseLight emits light and scatters nothing.
type DiffuseLight struct{ Colour vec.Vec3 }

func (m DiffuseLight) Scatter(Ray, Hit, *rand.Rand) (Ray, vec.Vec3, bool) {
	return Ray{}, vec.Vec3{}, false
}

func (m DiffuseLight) Emitted() vec.Vec3 { return m.Colour }
