package geo

import (
	"bytes"
	"fmt"
	"io"
	"math"
)

// Path represents a set of points to be thought of as a polyline.
type Path struct {
	points []Point
}

func NewPath() *Path {
	p := &Path{}
	p.points = make([]Point, 0, 1000)

	return p
}

// Transform applies a given projection or inverse projection to all
// the points in the path.
func (p *Path) Transform(projection func(*Point) *Point) *Path {
	for i := range p.points {
		projection(&p.points[i])
	}

	return p
}

// Reduce the path using Douglas Peuckered to the given threshold.
// Modifies the existing path.
func (p *Path) Reduce(threshold float64) *Path {
	mask := make([]byte, p.Length())

	p.workerReduce(0, p.Length()-1, threshold, mask)

	count := 0
	for i, v := range mask {
		if v == 1 {
			p.points[count] = p.points[i]
			count++
		}
	}

	p.points = p.points[:count]
	return p
}

func (p *Path) workerReduce(start, end int, threshold float64, mask []byte) {
	mask[start] = 1
	mask[end] = 1

	l := Line{p.points[start], p.points[end]}

	maxDist := 0.0
	maxIndex := 0
	for i := start + 1; i < end; i++ {
		dist := l.DistanceFrom(&p.points[i])

		if dist > maxDist {
			maxDist = dist
			maxIndex = i
		}
	}

	if maxDist > threshold {
		p.workerReduce(start, maxIndex, threshold, mask)
		p.workerReduce(maxIndex, end, threshold, mask)
	}
}

// Decode is the inverse of Encode. It takes a string encoding of path
// and returns the actual path it represents. Factor defaults to 1.0e5,
// the same used by Google for polyline encoding.
func Decode(encoded string, factor ...int) *Path {
	var count, index int

	f := 1.0e5
	if len(factor) != 0 {
		f = float64(factor[0])
	}

	p := NewPath()
	tempLatLng := [2]int{0, 0}

	for index < len(encoded) {
		var result int
		var b int = 0x20
		var shift uint

		for b >= 0x20 {
			b = int(encoded[index]) - 63
			index++

			result |= (b & 0x1f) << shift
			shift += 5
		}

		// sign dection
		if result&1 != 0 {
			result = ^(result >> 1)
		} else {
			result = result >> 1
		}

		if count%2 == 0 {
			result += tempLatLng[0]
			tempLatLng[0] = result
		} else {
			result += tempLatLng[1]
			tempLatLng[1] = result

			p.Push(&Point{float64(tempLatLng[1]) / f, float64(tempLatLng[0]) / f})
		}

		count++
	}

	return p
}

// Encode with Google polyline encode the path into a string.
// Factor defaults to 1.0e5, the same used by Google for polyline encoding.
func (p *Path) Encode(factor ...int) string {
	f := 1.0e5
	if len(factor) != 0 {
		f = float64(factor[0])
	}

	var pLat int
	var pLng int

	var result bytes.Buffer

	for _, p := range p.points {
		lat5 := int(p.Lat() * f)
		lng5 := int(p.Lng() * f)

		deltaLat := lat5 - pLat
		deltaLng := lng5 - pLng

		pLat = lat5
		pLng = lng5

		result.WriteString(encodeSignedNumber(deltaLat))
		result.WriteString(encodeSignedNumber(deltaLng))
	}

	return result.String()
}

func encodeSignedNumber(num int) string {
	shiftedNum := num << 1

	if num < 0 {
		shiftedNum = ^shiftedNum
	}

	return encodeNumber(shiftedNum)
}

func encodeNumber(num int) string {
	result := ""

	for num >= 0x20 {
		result += string((0x20 | (num & 0x1f)) + 63)
		num >>= 5
	}

	result += string(num + 63)

	return result
}

// TotalDistance computes the total distance in the units of the points
func (p *Path) TotalDistance() float64 {
	sum := 0.0

	loopTo := len(p.points) - 1
	for i := 0; i < loopTo; i++ {
		sum += p.points[i].DistanceFrom(&p.points[i+1])
	}

	return sum
}

// GeoTotalDistance computes the total distance using spherical geometry
func (p *Path) GeoTotalDistance(haversine ...bool) float64 {
	yesgeo := yesHaversine(haversine)
	sum := 0.0

	loopTo := len(p.points) - 1
	for i := 0; i < loopTo; i++ {
		sum += p.points[i].GeoDistanceFrom(&p.points[i+1], yesgeo)
	}

	return sum
}

// DistanceFrom computes an O(n) distance from the path. Loops over every
// subline to find the minimum distance.
func (p *Path) DistanceFrom(point *Point) float64 {
	dist := math.Inf(1)

	loopTo := len(p.points) - 1
	for i := 0; i < loopTo; i++ {
		l := &Line{p.points[i], p.points[i+1]}
		dist = math.Min(l.DistanceFrom(point), dist)
	}

	return dist
}

func (p *Path) Bounds() *Bound {
	if len(p.points) == 0 {
		return NewBound(0, 0, 0, 0)
	}

	minX := math.Inf(1)
	minY := math.Inf(1)

	maxX := math.Inf(-1)
	maxY := math.Inf(-1)

	for _, v := range p.points {
		minX = math.Min(minX, v.X())
		minY = math.Min(minY, v.Y())

		maxX = math.Max(maxX, v.X())
		maxY = math.Max(maxY, v.Y())
	}

	return NewBound(maxY, minY, maxX, minX)
}

// SetAt updates a position at i along the path.
// Panics if index is out of range.
func (p *Path) SetAt(index int, point *Point) *Path {
	if index >= len(p.points) || index < 0 {
		panic(fmt.Sprintf("geo: set index out of range, requested: %d, length: %d", index, len(p.points)))
	}
	p.points[index] = *point
	return p
}

// GetAt returns the Point at i. Return nil if index out of range
func (p *Path) GetAt(i int) *Point {
	if i >= len(p.points) {
		return nil
	}

	return &p.points[i]
}

// InsertAt inserts a Point at i along the path.
// Panics if index is out of range.
func (p *Path) InsertAt(index int, point *Point) *Path {
	if index > len(p.points) || index < 0 {
		panic(fmt.Sprintf("geo: insert index out of range, requested: %d, length: %d", index, len(p.points)))
	}

	if index == len(p.points) {
		p.points = append(p.points, *point)
		return p
	}

	p.points = append(p.points, Point{})
	copy(p.points[index+1:], p.points[index:])
	p.points[index] = *point

	return p
}

// RemoveAt removes a Point at i along the path.
// Panics if index is out of range.
func (p *Path) RemoveAt(index int) *Path {
	if index >= len(p.points) || index < 0 {
		panic(fmt.Sprintf("geo: remove index out of range, requested: %d, length: %d", index, len(p.points)))
	}

	p.points = append(p.points[:index], p.points[index+1:]...)
	return p
}

func (p *Path) Push(point *Point) *Path {
	p.points = append(p.points, *point)
	return p
}

func (p *Path) Pop() *Point {
	if len(p.points) == 0 {
		return nil
	}

	x := p.points[len(p.points)-1]
	p.points = p.points[:len(p.points)-1]

	return &x
}

func (p *Path) Length() int {
	return len(p.points)
}

func (p *Path) Clone() *Path {
	n := NewPath()
	n.points = append(n.points, p.points[:]...)
	return n
}

// WriteOffFile writes an Object File Format representation of
// the points of the path to the writer provided. This is for viewing
// in MeshLab or something like that. You should close the
// writer yourself after this function returns.
// http://segeval.cs.princeton.edu/public/off_format.html
func (p *Path) WriteOffFile(w io.Writer) {

	w.Write([]byte("OFF\n"))
	w.Write([]byte(fmt.Sprintf("%d 0 0\n", p.Length())))

	for i := range p.points {
		w.Write([]byte(fmt.Sprintf("%f %f 0\n", p.points[i][0], p.points[i][1])))
	}
}