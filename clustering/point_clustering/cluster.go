package point_clustering

import "github.com/paulmach/go.geo"

// A Pointer is the interface for something that can be point clustered.
// Basically anything that can be boiled down to a single point.
type Pointer interface {
	// CenterPoint is kind of a weird name, but it's meant to not overlap
	// with any stuct attributes.
	CenterPoint() *geo.Point // lng/lat, or other, point
}

// A Cluster is a cluster of pointers plus their centroid.
// It defines a center/centroid for easy centroid distance computation.
type Cluster struct {
	Centroid *geo.Point
	Pointers []Pointer
}

// NewPointCluster creates the point cluster and finds the center of the given pointers.
func NewCluster(pointers ...Pointer) *Cluster {
	var (
		sumX, sumY float64
		count      int
	)

	c := &Cluster{
		Pointers: pointers,
	}

	if len(pointers) == 0 {
		c.Centroid = geo.NewPoint(0, 0)
		return c
	}

	if len(pointers) == 1 {
		c.Centroid = pointers[0].CenterPoint().Clone()
		return c
	}

	// find the center/centroid of multiple points
	for _, pointer := range c.Pointers {
		cp := pointer.CenterPoint()

		sumX += cp.X()
		sumY += cp.Y()
		count++
	}
	c.Centroid = geo.NewPoint(sumX/float64(count), sumY/float64(count))

	return c
}

// NewClusterWithCentroid creates a point cluster stub from the given centroid
// and optional pointers.
func NewClusterWithCentroid(centroid *geo.Point, pointers ...Pointer) *Cluster {
	return &Cluster{
		Centroid: centroid.Clone(),
		Pointers: pointers,
	}
}

// Merge merges the given point clusters into the current cluster and returns.
// It mutates the base cluster. Updates the centroid.
func (c *Cluster) Merge(c2 *Cluster) {
	c.Centroid = geo.NewLine(c.Centroid, c2.Centroid).Interpolate(1 - float64(len(c.Pointers))/float64(len(c2.Pointers)+len(c.Pointers)))
	c.Pointers = append(c.Pointers, c2.Pointers...)

	return
}
