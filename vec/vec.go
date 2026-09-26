// Package vec provides the 3-component vector arithmetic the ray tracer is
// built on. Vec3 is a value type: operations return new vectors rather than
// mutating, which keeps them safe to share across rendering goroutines.
package vec

import "math"

// Vec3 is a point, direction or colour in 3-space depending on context.
type Vec3 struct{ X, Y, Z float64 }

func New(x, y, z float64) Vec3 { return Vec3{x, y, z} }

func (v Vec3) Add(w Vec3) Vec3      { return Vec3{v.X + w.X, v.Y + w.Y, v.Z + w.Z} }
func (v Vec3) Sub(w Vec3) Vec3      { return Vec3{v.X - w.X, v.Y - w.Y, v.Z - w.Z} }
func (v Vec3) Mul(w Vec3) Vec3      { return Vec3{v.X * w.X, v.Y * w.Y, v.Z * w.Z} }
func (v Vec3) Scale(t float64) Vec3 { return Vec3{v.X * t, v.Y * t, v.Z * t} }
func (v Vec3) Neg() Vec3            { return Vec3{-v.X, -v.Y, -v.Z} }

func (v Vec3) Dot(w Vec3) float64 { return v.X*w.X + v.Y*w.Y + v.Z*w.Z }

func (v Vec3) Cross(w Vec3) Vec3 {
	return Vec3{
		v.Y*w.Z - v.Z*w.Y,
		v.Z*w.X - v.X*w.Z,
		v.X*w.Y - v.Y*w.X,
	}
}

func (v Vec3) LenSq() float64 { return v.Dot(v) }
func (v Vec3) Len() float64   { return math.Sqrt(v.LenSq()) }

// Unit returns v scaled to length 1. A zero vector has no direction, so it
// is returned unchanged rather than producing NaNs that would propagate
// silently through the whole render.
func (v Vec3) Unit() Vec3 {
	l := v.Len()
	if l == 0 {
		return v
	}
	return v.Scale(1 / l)
}

// NearZero reports whether v is close enough to zero that using it as a
// direction would be numerically meaningless. Scatter directions that cancel
// out must be caught, or they produce NaN normals downstream.
func (v Vec3) NearZero() bool {
	const eps = 1e-8
	return math.Abs(v.X) < eps && math.Abs(v.Y) < eps && math.Abs(v.Z) < eps
}

// Reflect mirrors v about the surface normal n, which must be a unit vector.
func Reflect(v, n Vec3) Vec3 { return v.Sub(n.Scale(2 * v.Dot(n))) }

// Refract bends v through a surface with normal n according to Snell's law.
// ratio is the incident index divided by the transmitted index.
func Refract(v, n Vec3, ratio float64) Vec3 {
	cosTheta := math.Min(v.Neg().Dot(n), 1.0)
	perp := v.Add(n.Scale(cosTheta)).Scale(ratio)
	parallel := n.Scale(-math.Sqrt(math.Abs(1.0 - perp.LenSq())))
	return perp.Add(parallel)
}

// Lerp linearly interpolates from a to b as t goes 0 to 1.
func Lerp(a, b Vec3, t float64) Vec3 { return a.Scale(1 - t).Add(b.Scale(t)) }
