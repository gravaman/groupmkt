#!/bin/bash

# SOCKSPort [number] to bind localhost:[number] for local connections
# RunAsDaemon to start process in the background
# DataDirectory for keeping keys/etc; each tor process needs a separate data dir
# ControlPort tor listens on for controller apps

base_socks_port=9050
package="tor_runner"
conns=1
verbose=false

# handle cli opts
while test "$#" -gt 0; do
	case "$1" in
		-h|--help)
			echo "$package - initialize tor network"
			echo " "
			echo "$package [options]"
			echo " "
			echo "-h, --help		display help"
			echo "-c, --conn		connection count"
			exit 0
			;;
		-v|--verbose)
			verbose=true
			;;
		-c|--conn)
			shift
			conns=$1
			;;
	esac
	shift
done

# display configuration
if [ "$verbose" = true ]; then
	echo "$package configuration"
	echo " "
	echo "verbose: $verbose"
	echo "connections: $conns"
fi

# bind new port for each conn
for (( i=0; i<conns; i++ ))
do
	socks_port=$((base_socks_port+i))

	# create tor data dir if doesn't already exist
	if [ ! -d "data/tor$i" ]; then
		mkdir -p "data/tor$i"
	fi

	if [ "$verbose" = true ]; then
		echo "Running: tor --RunAsDaemon 1 --SocksPort $socks_port --DataDirectory ${PWD}/data/tor$i --PidFile ${PWD}/data/tor${i}.pid"
	fi

	# bind port
	tor --RunAsDaemon 1 --SocksPort $socks_port --DataDirectory "${PWD}/data/tor$i" --PidFile "${PWD}/data/tor${i}.pid"
done

