upnp
-----

[![GoDoc](https://godoc.org/github.com/lukechampine/upnp?status.svg)](https://godoc.org/github.com/lukechampine/upnp)
[![Go Report Card](http://goreportcard.com/badge/github.com/lukechampine/upnp)](https://goreportcard.com/report/github.com/lukechampine/upnp)

```
go get github.com/lukechampine/upnp
```

Yep, it's another package for forwarding ports and discovering your external IP
address. This one has no dependencies and is ~1 MB smaller than
[huin/goupnp](https://github.com/huin/goupnp).

## Usage

```go
// scan for router(s)
devices, _ := upnp.Discover(context.Background())
d := devices[0]

// connect to a previously-scanned router
routerURL := d.Location()
d, _ = upnp.Connect(context.Background(), routerURL)

// forward/clear a port
println(d.IsForwarded(15000, "TCP")) // false
d.Forward(15000, "TCP", "example description")
println(d.IsForwarded(15000, "TCP")) // true
d.Clear(15000, "TCP")
println(d.IsForwarded(15000, "TCP")) // false

// get external IP
ip, _ := d.ExternalIP()
println(ip)
```