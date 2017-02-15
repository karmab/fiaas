source ~/keystonerc_admin
neutron subnet-create --name dev --allocation-pool start=192.168.70.2,end=192.168.70.254 --disable-dhcp --gateway 192.168.70.1 external 192.168.70.0/24
neutron subnet-create --name pre --allocation-pool start=192.168.71.2,end=192.168.71.254 --disable-dhcp --gateway 192.168.71.1 external 192.168.71.0/24
neutron subnet-create --name pro --allocation-pool start=192.168.72.2,end=192.168.72.254 --disable-dhcp --gateway 192.168.72.1 external 192.168.72.0/24
