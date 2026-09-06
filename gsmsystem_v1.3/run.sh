#!/bin/bash

### get the conf path
current_path=$(pwd)

## config path

# log path
sipauthserve_log=(/OpenBTS/log/sipauthserve_run.log)
smqueue_log=(/OpenBTS/log/smqueue_run.log)
asterisk_log=(/OpenBTS/log/asterisk_run.log)
openbts_log=(/OpenBTS/log/openbts_run.log)
transceiver_log=(/OpenBTS/log/transceiver_runlog)

openbts_pid=$(ps -aux | grep -v 'grep'  | grep /OpenBTS/OpenBTS | awk '{print $2}')
transceiver_pid=$(ps -aux | grep -v 'grep'  | grep transceiver | awk '{print $2}')
smqueue_pid=$(ps -aux | grep -v 'grep'  | grep smqueue | awk '{print $2}')
asterisk_pid=$(ps -aux | grep -v 'grep'  | grep asterisk | awk '{print $2}')
sipauthserve_pid=$(ps -aux | grep -v 'grep'  | grep sipauthserve | awk '{print $2}')

echo -e "\033[32m[info] asterisk:${asterisk_pid}, sipauthserver:${sipauthserve_pid}, smqueue:${smqueue_pid}\033[0m"
# start asterisk if pid is null.
if [ -z "$asterisk_pid" ]
then
	echo -e "\033[32m[info] start to start asterisk......\033[0m"
	asterisk -f -g  > $asterisk_log 2>&1 &
	asterisk_pid=$(ps -aux  | grep -v 'grep' | grep asterisk | awk '{print $2}')
	echo -e "\033[32m[success] Start asterisk, pid=$asterisk_pid, the log is in $asterisk_log\033[0m"
else
    echo -e "\033[31m[error] asterisk pid: $asterisk_pid is running......\033[0m"
fi

# start sipauthserve if pid is null.
if [ -z "$sipauthserve_pid" ] 
then
	echo -e "\033[32m[info] start to start sipauthserve......\033[0m"
    sipauthserve > $sipauthserve_log 2>&1 &
	sipauthserve_pid=$(ps -aux  | grep -v 'grep' | grep sipauthserve | awk '{print $2}')
	echo -e "\033[32m[success] Start sipauthserve, pid=$sipauthserve_pid, the log is in $sipauthserve_log\033[0m"
else
    echo -e "\033[31m[error] sipauthserve pid: $sipauthserve_pid is running......\033[0m"
fi

# start smqueue if pid is null
if [ -z "$smqueue_pid" ]
then
	echo -e "\033[32m[info] start to start smqueue......\033[0m"
	smqueue  > $smqueue_log 2>&1 &
	smqueue_pid=$(ps -aux  | grep -v 'grep' | grep smqueue | awk '{print $2}')
	echo -e "\033[32m[success] Start smqueue, pid=$smqueue_pid, the log is in /var/log/smqueue.log\033[0m"
else
    echo -e "\033[31m[error] smqueue pid: $smqueue_pid is running......\033[0m"
fi

# Check if USRP is connected.
usrp_connect_count=0 # setting a count,if waitting for too long(more than 60s) ,stop run
max_usrp_connect_count=12
echo -e "\033[32m[info] Starting to check if USRP B210 is connected......\033[0m"
device_info=$(uhd_find_devices | grep B210)
while [ -z "$device_info" ] # if null
do
	usrp_connect_count=$((usrp_connect_count+1))
	if [ "$usrp_connect_count" -eq "$max_usrp_connect_count" ]; 
	then
		echo -e "\033[31m[error] Waitting for USRP to connect more th 60s, exit......\033[0m"
		exit
	fi
	echo -e "\033[31m[error] No USRP B210 found......\033[0m"
	echo -e "\033[31m[error] Please check if thse device is connected......\033[0m"
	echo -e "\033[31m[error] Waiting for the device to connect......\033[0m"
	sleep 5
	device_info=$(uhd_find_devices | grep B210)
done
# Device connected successfully.
echo -e "\033[32m[success] ${device_info} connect success......\033[0m"

# start openbts
if [ -z "$openbts_pid" ] && [ -z "$transceiver_pid" ] # if null
then
	flag=0
	max_flag=10
	while [ "$flag" -lt "$max_flag" ]
	do
		flag_count=$((flag_count+1))
		
		# rm OpenBTS.pid
		echo -e "\033[32m[info] Starting to check /var/OpenBTS.pid\033[0m"
		if [ -e "/var/run/OpenBTS.pid" ]; 
		then
			echo -e "\033[31m[error] OpenBTS.pid is exist, start to delete it......\033[0m"
			rm /var/run/OpenBTS.pid
		else
			echo -e "\033[32m[info] OpenBTS.pid doesn't exist, continue start......\033[0m"
		fi

		# start OpenBTS
		echo -e "\033[32m[info] Everything is OK, start to run OpenBTS......\033[0m"
		/OpenBTS/OpenBTS > $openbts_log 2>&1 &
		
		# If openbts or transceiver is running, the following error may occur. If detected, kill run.sh,rm openbts.pid.
		echo -e "\033[32m[info] Startting to check an instance of /OpenBTS is already running......\033[0m"
		openbts_instance_status=$(cat $openbts_log | grep "An instance of /OpenBTS is already running.")
		if [ -n "$openbts_instance_status" ] # if not null
		then
			echo $openbts_instance_status
			# restart OpenBTS
			# rm OpenBTS.pid
			echo -e "\033[32m[info] Starting to check /var/OpenBTS.pid again......\033[0m"
			if [ -e "/var/run/OpenBTS.pid" ]; 
			then
				echo -e "\033[31m[error] OpenBTS.pid is exist, start to delete it......\033[0m"
				rm /var/run/OpenBTS.pid
			else
				echo -e "\033[32m[info] OpenBTS.pid doesn't exist, continue start......\033[0m"
			fi
		else
			echo -e "\033[32m[success]There has no instance of OpenBTS is running, continue to start......\033[0m"
		fi

		# check openbts start log to make sure openbts is starting....
		
		# get openbts system status
		openbts_status=$(cat $openbts_log | grep "system ready")
		# Check if OpenBTS started successfully, use while waitting for 20s , if more than 10,break, and auto goes into restart.
		system_ready_count=0 # setting a count,if waitting for too long(more than 10s) ,stop run
		max_system_ready_count=20
		while [ -z "$openbts_status" ] # while null
		do
			if [ "$system_ready_count" -eq "$max_system_ready_count" ]
			then
				echo -e "\033[31m[error]Waitting for more than $max_system_ready_count s, start to restart OpenBTS......\033[0m"
				break
			fi
			echo -e "\033[32m[info]Waitting $system_ready_count for system ready......\033[0m"
			sleep 1 # sleep for 1s to wait for system ready
			system_ready_count=$((system_ready_count+1))
			# get openbts system status
			openbts_status=$(cat $openbts_log | grep "system ready")
			if [ -n "$openbts_status" ]
			then
				echo -e "\033[32m[success]System ready......\033[0m"
				break
			fi
		done

		if [ -z "$openbts_status" ] # if null
		then
			# restart OpenBTS
			# rm OpenBTS.pid
			echo -e "\033[32m[info] Starting to check /var/OpenBTS.pid\033[0m"
			if [ -e "/var/run/OpenBTS.pid" ]; 
			then
				echo -e "\033[31m[error] OpenBTS.pid is exist, start to delete it......\033[0m"
				rm /var/run/OpenBTS.pid
			else
				echo -e "\033[32m[info] OpenBTS.pid doesn't exist, continue start......\033[0m"
			fi

			openbts_pid=$(ps -aux | grep -v 'grep'  | grep /OpenBTS/OpenBTS | awk '{print $2}')
			transceiver_pid=$(ps -aux | grep -v 'grep'  | grep transceiver | awk '{print $2}')
			if [ ! -n "$openbts_pid" ]
			then
				echo -e "\033[31m[error]Start failed, cant find OpenBTS's pid......\033[0m"
			else
				kill -9 $openbts_pid
				echo -e "\033[31m[error]Start failed, restarting now......\033[0m"
			fi
			if [ ! -n "$transceiver_pid" ] 
			then
				echo -e "\033[32m[success] transceiver have been killed......\033[0m"
			else
				echo -e "\033[32m[info] transceiver pid: $transceiver_pid are stopping......\033[0m"
				kill -9 $transceiver_pid
				echo -e "\033[32m[success] transceiver close complete......\033[0m"
			fi
		else
			transceiver_pid=$(ps -aux | grep -v 'grep'  | grep transceiver | awk '{print $2}')
			if [ -n "$transceiver_pid" ]
			then
				openbts_pid=$(ps -aux | grep -v 'grep'  | grep /OpenBTS/OpenBTS | awk '{print $2}')
				echo -e "\033[32m[info] Start OpenBTS, pid=$openbts_pid, the log is in $openbts_log\033[0m"
				exit
			else
				echo -e "\033[31m[error]Start failed, Unkonow error......\033[0m"
				exit
			fi
			break
		fi
	done
	echo -e "\033[31m[error] Too long to start, please check system, exitting......\033[0m"
else
    echo -e "\033[32m[success] openbts pid: $openbts_pid and transceiver pid: $transceiver_pid are running......\033[0m"
fi