// Package upnp provides a simple interface for forwarding ports and discovering
// external IP addresses on UPnP-enabled routers.
package upnp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"lukechampine.com/upnp/internal/goupnp"
)

// A Device can forward ports and discover its external IP.
type Device struct {
	internalIP string
	client     goupnp.IGDClient
}

// Forward forwards the specified port for the specified protocol, which must be
// "TCP" or "UDP".
func (d Device) Forward(port uint16, proto string, desc string) error {
	return d.client.AddPortMapping(goupnp.AddPortMappingRequest{
		NewExternalPort:           port,
		NewProtocol:               proto,
		NewInternalPort:           port,
		NewInternalClient:         d.internalIP,
		NewEnabled:                true,
		NewPortMappingDescription: desc,
		NewLeaseDuration:          0,
	})
}

// IsForwarded returns true if the specified port is forwarded to this host.
func (d Device) IsForwarded(port uint16, proto string) bool {
	resp, _ := d.client.GetSpecificPortMappingEntry(goupnp.GetSpecificPortMappingEntryRequest{
		NewExternalPort: port,
		NewProtocol:     proto,
	})
	return resp.NewEnabled && resp.NewInternalClient == d.internalIP
}

// Clear un-forwards a port. No error is returned if the port is not forwarded.
func (d Device) Clear(port uint16, proto string) error {
	err := d.client.DeletePortMapping(goupnp.DeletePortMappingRequest{
		NewExternalPort: port,
		NewProtocol:     proto,
	})
	if err != nil && strings.Contains(err.Error(), "NoSuchEntryInArray") {
		err = nil
	}
	return err
}

// ExternalIP returns the router's external IP.
func (d Device) ExternalIP() (string, error) {
	resp, err := d.client.GetExternalIPAddress()
	return resp.NewExternalIPAddress, err
}

// Location returns the URL of the device.
func (d Device) Location() string {
	return d.client.Location()
}

func getInternalIP(loc string) (string, error) {
	// NOTE: this function makes a lot of syscalls, and we call it for *every*
	// ServiceClient we discover, so it may be tempting to just fetch the set of
	// interfaces once and cache them thereafter. Don't do this! Despite the
	// syscalls, this is not a slow function; you can call it ~20,000 times per
	// second, and the vast majority of programs will only need to call it a
	// handful of times at startup. Better to eat the cost and avoid potential
	// surprising behavior caused by a stale cache.

	baseURL, err := url.Parse(loc)
	if err != nil {
		return "", err
	}
	devAddr, err := net.ResolveUDPAddr("udp4", baseURL.Host)
	if err != nil {
		return "", err
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			if x, ok := addr.(*net.IPNet); ok && x.Contains(devAddr.IP) {
				return x.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("could not find local address in same net as %v", devAddr)
}

// DiscoverAll scans the local network for Devices.
func DiscoverAll() (<-chan Device, error) {
	locations, err := goupnp.SSDP()
	if err != nil {
		return nil, err
	}
	ch := make(chan Device)
	go doDiscoverAll(locations, ch)
	return ch, nil
}

func doDiscoverAll(locations <-chan string, devices chan<- Device) {
	var wg sync.WaitGroup
	for url := range locations {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cs, _ := goupnp.IGDClientsByURL(ctx, url)
			for _, c := range cs {
				if ip, err := getInternalIP(c.Location()); err == nil {
					devices <- Device{ip, c}
				}
			}
		}(url)
	}
	wg.Wait()
	close(devices)
}

// Discover scans the local network for Devices, reurning the first Device
// found.
func Discover(ctx context.Context) (Device, error) {
	devices, err := DiscoverAll()
	if err != nil {
		return Device{}, err
	}
	// ensure we fully consume channel
	defer func() {
		go func() {
			for range devices {
			}
		}()
	}()
	select {
	case d, ok := <-devices:
		if !ok {
			return Device{}, errors.New("no UPnP-enabled gateway found")
		}
		return d, nil
	case <-ctx.Done():
		return Device{}, ctx.Err()
	}
}

// Connect connects to the router service specified by deviceURL. Generally,
// Connect should only be called with URLs returned by (Device).Location.
func Connect(ctx context.Context, deviceURL string) (Device, error) {
	clients, err := goupnp.IGDClientsByURL(ctx, deviceURL)
	if err != nil {
		return Device{}, err
	}
	if len(clients) == 0 {
		return Device{}, fmt.Errorf("no UPnP-enabled gateway found at %v", deviceURL)
	} else if len(clients) > 1 {
		return Device{}, fmt.Errorf("multiple UPnP-enabled gateways found at %v", deviceURL)
	}
	c := clients[0]
	ip, err := getInternalIP(c.Location())
	if err != nil {
		return Device{}, err
	}
	return Device{ip, c}, nil
}
