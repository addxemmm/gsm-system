#!/bin/bash
smqueue_pid=$(ps -aux | grep -v 'grep'  | grep smqueue | awk '{print $2}')
asterisk_pid=$(ps -aux | grep -v 'grep'  | grep asterisk | awk '{print $2}')
sipauthserve_pid=$(ps -aux | grep -v 'grep'  | grep sipauthserve | awk '{print $2}')
transceiver_pid=$(ps -aux | grep -v 'grep'  | grep transceiver | awk '{print $2}')

# Stop smqueue
if [ ! -n "$smqueue_pid" ] 
then
    echo "smqueue have been killed......"
else
    echo "smqueue pid: $smqueue_pid is stopping......"
    kill -9 $smqueue_pid
    echo "smqueue close complete......"
fi

# Stop asterisk
if [ ! -n "$asterisk_pid" ] 
then
    echo "asterisk have been killed......"
else
    echo "asterisk pid: $asterisk_pid is stopping......"
    kill -9 $asterisk_pid
    echo "asterisk close complete......"
fi

# Stop sipauthserve
if [ ! -n "$sipauthserve_pid" ] 
then
    echo "sipauthserve have been killed......"
else
    echo "sipauthserve pid: $sipauthserve_pid is stopping......"
    kill -9 $sipauthserve_pid
    echo "sipauthserve close complete......"
fi

# Stop OpenBTS
rm /var/run/OpenBTS.pid
openbts_pid=$(ps -aux | grep -v 'grep'  | grep /OpenBTS/OpenBTS | awk '{print $2}')
if [ ! -n "$openbts_pid" ] 
then
    echo "openbts have been killed......"
else
    echo "openbts pid: $openbts_pid are stopping......"
    kill -9 $openbts_pid   
    echo "openbts close complete......"
fi

if [ ! -n "$transceiver_pid" ] 
then
    echo "transceiver have been killed......"
else
    echo "transceiver pid: $transceiver_pid are stopping......"
    kill -9 $transceiver_pid
    echo "transceiver close complete......"
fi