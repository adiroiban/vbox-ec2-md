VirtualBox EC2 Metadata Service
===============================

This project tries to implement a simple HTTP server which can work with
VirtualBox in order to server cloud-init requests from the guest VMs
executed using VirtualBox.

It also provides a proxy for the VirtualBox Web Service so that you can
have a single HTTP endpoint to turn VirtualBox into a cloud provider.

Amazon EC2 metadata server is expected to be available on 169.254.169.254 on
port 80. To have it run as non-root you can redirect the port 80::

    iptables -t nat -A OUTPUT -p tcp \
        -d 169.254.169.254/32 --dport 80  -j DNAT \
        --to-destination 169.254.169.254:18082