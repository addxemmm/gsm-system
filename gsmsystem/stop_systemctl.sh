#!/bin/bash
asterisk_status = $(systemctl status asterisk | grep Active | awk '{print $2}')
sipauthserve_status = $(systemctl status sipauthserve | grep Active | awk '{print $2}')
smqueue_status = $(systemctl status smqueue | grep Active | awk '{print $2}')

# Stop asterisk,sipauthserve,smqueue, Just shut down the asterisk service, other services can be shut down at the same time.
if ["$asterisk_status" = "active" | "$sipauthserve_status" = "active" | "$smqueue_status" = "active" ] 
then
    echo "start to stop asterisk sipauthserve and smqueue......"
    systemctl stop asterisk
else
    echo "asterisk,sipauthserve and smqueue are not running......"
fi

# Stop OpenBTS
rm /var/run/OpenBTS.pid
openbts_pid=$(ps -aux | grep -v 'grep'  | grep /OpenBTS/OpenBTS | awk '{print $2}')
transceiver_pid=$(ps -aux | grep -v 'grep'  | grep transceiver | awk '{print $2}')
if [ ! -n "$transceiver_pid" ] 
then
    echo "transceiver have been killed......"
else
    echo "transceiver pid: $transceiver_pid are stopping......"
    kill -9 $transceiver_pid
    echo "transceiver close complete......"
fi
if [ ! -n "$openbts_pid" ] 
then
    echo "openbts have been killed......"
else
    echo "openbts pid: $openbts_pid are stopping......"
    /OpenBTS/OpenBTSCLI -c tmsis clear
    echo "clean tmsis..."
    kill -9 $openbts_pid   
    echo "openbts close complete......"
fi