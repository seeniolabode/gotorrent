package udp

import (
	"github.com/seeniolabode/gotorrent/internals/tracker"
)

func (c *UDPTrackerClient) Run() (tracker.TrackingResult, error) {
	err := c.init()
	defer c.close()

	if err != nil {
		return tracker.TrackingResult{}, err
	}

	err = c.connect()

	if err != nil {
		return tracker.TrackingResult{}, err
	}

	res, err := c.announce()

	if err != nil {
		return tracker.TrackingResult{}, err
	}

	c.result = res

	return res, nil

}
