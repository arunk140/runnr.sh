#!/bin/bash

# Get kernel version
kernel=$(uname -r)

# Get package counts for installed package managers
if command -v dpkg &> /dev/null; then
    dpkg_count=$(dpkg -l | grep '^ii' | wc -l)
    dpkg_count=$((dpkg_count-1))
    package_counts="dpkg:$dpkg_count"
fi

if command -v nix-env &> /dev/null; then
    nix_count=$(nix-store -q --requisites ~/.nix-profile | wc -l)
    package_counts="$package_counts nix-user:$nix_count"
fi

if command -v pkg &> /dev/null; then
    pkg_count=$(pkg info | wc -l)
    pkg_count=$((pkg_count-1))
    package_counts="$package_counts pkg:$pkg_count"
fi

if command -v flatpak &> /dev/null; then
    flatpak_count=$(flatpak list | wc -l)
    package_counts="$package_counts flatpak:$flatpak_count"
fi

if command -v snap &> /dev/null; then
    snap_count=$(snap list | wc -l)
    package_counts="$package_counts snap:$snap_count"
fi

# Get CPU info
cpu=$(lscpu | grep 'Model name' | awk -F ': ' '{print $2}' | sed -e 's/^[[:space:]]*//')

# Get GPU info
gpu=$(lspci | grep 'VGA compatible controller' | awk -F ': ' '{print $2}')

# Get memory info
memory=$(free -m | grep 'Mem:' | awk '{print $3 "MiB / " $2 "MiB"}')

# Get distro info
distro=$(cat /etc/*release | grep '^PRETTY_NAME' | awk -F '=' '{print $2}' | tr -d '[:]"')

# Print output in desired format
echo "sysinfo:"
echo "kernel: $kernel"
echo "packages: $package_counts"
echo "cpu: $cpu"
echo "gpu: $gpu"
echo "memory: $memory"
echo "distro: $distro"
echo ""