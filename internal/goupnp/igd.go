package goupnp

import (
	"context"
)

type GetSpecificPortMappingEntryRequest struct {
	NewRemoteHost   string
	NewExternalPort uint16
	NewProtocol     string
}

type GetSpecificPortMappingEntryResponse struct {
	NewInternalPort           uint16
	NewInternalClient         string
	NewEnabled                bool
	NewPortMappingDescription string
	NewLeaseDuration          string
}

type AddPortMappingRequest struct {
	NewRemoteHost             string
	NewExternalPort           uint16
	NewProtocol               string
	NewInternalPort           uint16
	NewInternalClient         string
	NewEnabled                bool
	NewPortMappingDescription string
	NewLeaseDuration          uint32
}

type DeletePortMappingRequest struct {
	NewRemoteHost   string
	NewExternalPort uint16
	NewProtocol     string
}

type GetExternalIPAddressResponse struct {
	NewExternalIPAddress string
}

type IGDClient struct {
	urlBase string
	srv     Service
}

func (igd IGDClient) performAction(actionName string, req interface{}, resp interface{}) error {
	return performSOAPAction(igd.urlBase+igd.srv.ControlURL, igd.srv.ServiceType, actionName, req, resp)
}

func (igd IGDClient) GetSpecificPortMappingEntry(req GetSpecificPortMappingEntryRequest) (resp GetSpecificPortMappingEntryResponse, err error) {
	err = igd.performAction("GetSpecificPortMappingEntry", req, &resp)
	return
}

func (igd IGDClient) AddPortMapping(req AddPortMappingRequest) error {
	return igd.performAction("AddPortMapping", req, nil)
}

func (igd IGDClient) DeletePortMapping(req DeletePortMappingRequest) error {
	return igd.performAction("DeletePortMapping", req, nil)
}

func (igd IGDClient) GetExternalIPAddress() (resp GetExternalIPAddressResponse, err error) {
	err = igd.performAction("GetExternalIPAddress", nil, &resp)
	return
}

func (igd IGDClient) Location() string {
	return igd.urlBase
}

func (igd IGDClient) ServiceType() string {
	return igd.srv.ServiceType
}

func DiscoverIGDClients(ctx context.Context) ([]IGDClient, error) {
	locations, err := SSDP(ctx)
	if err != nil {
		return nil, err
	}
	var clients []IGDClient
	for _, url := range locations {
		cs, _ := IGDClientsByURL(ctx, url)
		clients = append(clients, cs...)
	}
	return clients, nil
}

func IGDClientsByURL(ctx context.Context, url string) ([]IGDClient, error) {
	rd, err := DeviceByURL(ctx, url)
	if err != nil {
		return nil, err
	}

	var clients []IGDClient
	var visit func(Device)
	visit = func(d Device) {
		for _, srv := range d.Services {
			switch srv.ServiceType {
			case "urn:schemas-upnp-org:service:WANPPPConnection:1",
				"urn:schemas-upnp-org:service:WANIPConnection:1",
				"urn:schemas-upnp-org:service:WANIPConnection:2":
				clients = append(clients, IGDClient{rd.URLBase, srv})
			}
		}
		for _, d := range d.Devices {
			visit(d)
		}
	}
	visit(rd.Device)
	return clients, nil
}
