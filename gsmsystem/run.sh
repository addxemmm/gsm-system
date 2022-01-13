#!/bin/bash

### get the conf path
current_path=$(pwd)

## config path

# log path
sipauthserve_log=(/OpenBTS/log/sipauthserve.log)
smqueue_log=(/OpenBTS/log/smqueue.log)
asterisk_log=(/OpenBTS/log/asterisk.log)
openbts_log=(/OpenBTS/log/openbts.log)
transceiver_log=(/OpenBTS/log/transceiver.log)

openbts_pid=$(ps -aux | grep -v 'grep'  | grep /OpenBTS/OpenBTS | awk '{print $2}')
smqueue_pid=$(ps -aux | grep -v 'grep'  | grep smqueue | awk '{print $2}')
asterisk_pid=$(ps -aux | grep -v 'grep'  | grep asterisk | awk '{print $2}')
sipauthserve_pid=$(ps -aux | grep -v 'grep'  | grep sipauthserve | awk '{print $2}')
transceiver_pid=$(ps -aux | grep -v 'grep'  | grep transceiver | awk '{print $2}')

# start sipauthserve
if [ ! -n "$sipauthserve_pid" ] 
then
    systemctl start sipauthserve
	sipauthserve_pid=$(ps -aux  | grep -v 'grep' | grep sipauthserve | awk '{print $2}')
	echo "Start sipauthserve, pid=$sipauthserve_pid, the log is in $sipauthserve_log"
else
    echo "sipauthserve pid: $sipauthserve_pid is running......"
fi

# start smqueue
if [ ! -n "$smqueue_pid" ]
then
	systemctl start smqueue
	smqueue_pid=$(ps -aux  | grep -v 'grep' | grep smqueue | awk '{print $2}')
	echo "Start smqueue, pid=$smqueue_pid, the log is in /var/log/smqueue.log"
else
    echo "smqueue pid: $smqueue_pid is running......"
fi

# start asterisk
if [ ! -n "$asterisk_pid" ]
then
	systemctl start asterisk
	asterisk_pid=$(ps -aux  | grep -v 'grep' | grep asterisk | awk '{print $2}')
	echo "Start asterisk, pid=$asterisk_pid, the log is in $asterisk_log"
else
    echo "asterisk pid: $asterisk_pid is running......"
fi

# start openbts
if [ ! -n "$openbts_pid" ]
then
	flag=0
	while [ $flag == 0 ]
	do
		# Check if USRP is connected.
		device_info=$(uhd_find_devices | grep B210)
		while [ ${#device_info} == 0 ]
		do
			echo -e "\033[31mNo USRP B210 found......\033[0m"
			echo -e "\033[31mPlease check if thse device is connected......\033[0m"
			echo -e "\033[31mWaiting for the device to connect......\033[0m"
			sleep 5
			device_info=$(uhd_find_devices | grep B210)
		done
		# Device connected successfully.
		echo -e "\033[32m${device_info} connect success......\033[0m"

		# start OpenBTS
		/OpenBTS/OpenBTS > $openbts_log 2>&1 &

		openbts_status=$(cat $openbts_log | grep "system ready")
		test_message=$(cat $openbts_log | grep "Performing timer loopback test... pass")
		while [ ! -n "$test_message" ]
		do
			echo -e "\033[31mOpenBTS is starting,please wait......\033[0m"
			sleep 2
			test_message=$(cat $openbts_log | grep "Performing timer loopback test... pass")
		done
		sleep 3
		openbts_status=$(cat $openbts_log | grep "system ready")
		echo $openbts_status
		if [ ! -n "$openbts_status" ]
		then
			# restart OpenBTS
			openbts_pid=$(ps -aux | grep -v 'grep'  | grep /OpenBTS/OpenBTS | awk '{print $2}')
			transceiver_pid=$(ps -aux | grep -v 'grep'  | grep transceiver | awk '{print $2}')
			if [ ! -n "$openbts_pid" ]
			then
				echo -e "\033[31mStart failed, cant find OpenBTS's pid......\033[0m"
			else
				kill -9 $openbts_pid
				echo -e "\033[31mStart failed, restarting now......\033[0m"
			fi
			if [ ! -n "$transceiver_pid" ] 
			then
				echo "transceiver have been killed......"
			else
				echo "transceiver pid: $transceiver_pid are stopping......"
				kill -9 $transceiver_pid
				echo "transceiver close complete......"
			fi
		else
			flag=1
			openbts_pid=$(ps -aux | grep -v 'grep'  | grep /OpenBTS/OpenBTS | awk '{print $2}')
			echo "Start OpenBTS, pid=$openbts_pid, the log is in $openbts_log"
		fi
	done
else
    echo "openbts pid: $openbts_pid is running......"
fi