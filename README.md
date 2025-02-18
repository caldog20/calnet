# Calnet
Calnet is a peer-to-peer encrypted tunneling network project. It is similar to Tailscale, Netbird, and Wireguard. It uses Noise IK encryption for traffic between peers. 

## Control Plane
The control-plane of Calnet is a server package that runs an HTTP server and a STUN server. The HTTP server serves endpoints for the control-plane to manage peers and share peer information (node configuration, firewall rules, peer public keys, metadata, etc). The HTTP server also serves a websocket endpoint for relayed traffic when a peer-to-peer tunnel cannot be built between nodes. The REST API for management and frontend-usage is also served from the server package. 

Currently, the control server uses bbolt for backend storage. It will also have a Sqlite implementation that can be chosen from. 

OpenID/OAuth authentication can be enabled to provide access to the rest api as well as to register new nodes using SSO.

## Data Plane
The data-plane of Calnet is the node package. The node is the 'client' than runs on a machine to allow communication with other peers. The node runs as a daemon in userspace and hosts a local api to allow the cli client to run commands (login, up, down, etc). The node client communicate with the control-plane, pull it's node configuration (ID, IP Address, Routes, etc) then configure a UDP socket and local tunnel interface on the host. Routes will be installed for advertised network prefixes using the tunnel interface as a next-hop. The network prefix for the overlay is in CG-NAT space. The default prefix is 100.70.0.0/24 (this is configurable). 

When a node tries to communicate with another node (peer), the node will gather it's local connection candidates and exchange them with the peer through the control-plane. Both nodes will then attempt to 'ping' each other, attempting NAT traversal until it finds a successful communication path. While this process is ongoing, the node will use the relay to send the packet to the peer. If a peer-to-peer connection cannot be made (hard NATs, firewalls blocking), the relay will be used to continue to send packets to the peer. If traffic flow continues between the nodes, a peer-to-peer connection will periodically be attempted.

## More on usage and configuration to come.
