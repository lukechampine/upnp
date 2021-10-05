package goupnp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

type Service struct {
	ServiceType string `xml:"serviceType"`
	ControlURL  string `xml:"controlURL"`
}

type Device struct {
	DeviceType   string    `xml:"deviceType"`
	FriendlyName string    `xml:"friendlyName"`
	Services     []Service `xml:"serviceList>service,omitempty"`
	Devices      []Device  `xml:"deviceList>device,omitempty"`
}

type RootDevice struct {
	XMLName xml.Name `xml:"root"`
	URLBase string   `xml:"URLBase"`
	Device  Device   `xml:"device"`
}

func DeviceByURL(ctx context.Context, url string) (RootDevice, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return RootDevice{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		resp, _ := ioutil.ReadAll(resp.Body)
		return RootDevice{}, errors.New(string(resp))
	}

	var root RootDevice
	dec := xml.NewDecoder(resp.Body)
	dec.DefaultSpace = "urn:schemas-upnp-org:device-1-0"
	if err := dec.Decode(&root); err != nil {
		return RootDevice{}, fmt.Errorf("invalid response body: %w", err)
	}
	if root.URLBase == "" {
		root.URLBase = url
	}
	return root, nil
}

func SSDP() (<-chan string, error) {
	const maxWait = 2 * time.Second
	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return nil, err
	}
	conn.SetDeadline(time.Now().Add(maxWait + 100*time.Millisecond))
	ssdpUDP4Addr := &net.UDPAddr{IP: net.ParseIP("239.255.255.250"), Port: 1900}
	reqPacket := []byte(strings.Replace(fmt.Sprintf(`
M-SEARCH * HTTP/1.1
HOST: %v
MAN: "ssdp:discover"
MX: %v
ST: upnp:rootdevice

`[1:], ssdpUDP4Addr, int(maxWait.Seconds())), "\n", "\r\n", -1))
	const numSends = 3
	const sendInterval = 5 * time.Millisecond
	for i := 0; i < numSends; i++ {
		if _, err := conn.WriteTo(reqPacket, ssdpUDP4Addr); err != nil {
			return nil, fmt.Errorf("couldn't write SSDP packet: %w", err)
		}
		time.Sleep(sendInterval)
	}

	locs := make(chan string)
	go doSSDP(conn, locs)
	return locs, nil
}

func doSSDP(conn net.PacketConn, locs chan<- string) {
	defer conn.Close()
	defer close(locs)

	seen := make(map[string]bool)
	respPacket := make([]byte, 2048)
	r := bytes.NewReader(respPacket)
	br := bufio.NewReaderSize(r, len(respPacket))
	for {
		n, _, err := conn.ReadFrom(respPacket)
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Temporary() && !err.Timeout() {
				time.Sleep(5 * time.Millisecond)
				continue
			}
			return
		}
		r.Reset(respPacket[:n])
		br.Reset(r)
		response, err := http.ReadResponse(br, nil)
		if err != nil || response.StatusCode != 200 {
			continue
		}
		location, err := response.Location()
		if err != nil {
			continue
		}
		usn := response.Header.Get("USN")
		if usn == "" {
			usn = location.String()
		}
		if !seen[usn] {
			seen[usn] = true
			locs <- location.String()
		}
	}
}
