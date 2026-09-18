# kwikmesh

Manage Wireguard configurations for mesh nodes in a single YAML file.
The configuration for each endpoint includes both the Interface and Peer
information for that node as well as some additional fields describing
e.g. which peers are to be configured for that endpoint.

The output formats include the standard `wg-quick` INI-style as well
as OpenWRT's UCI config file format.
