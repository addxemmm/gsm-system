#!/bin/bash
smqueue_pid=$(ps -aux | grep -v 'grep'  | grep smqueue | awk '{print $2}')
asterisk_pid=$(ps -aux | grep -v 'grep'  | grep asterisk | awk '{print $2}')
sipauthserve_pid=$(ps -aux | grep -v 'grep'  | grep sipauthserve | awk '{print $2}')
transceiver_pid=$(ps -aux | grep -v 'grep'  | grep transceiver | awk '{print $2}')

# Stop smqueue if pid is not null.
if [ ! -n "$smqueue_pid" ] 
then
    echo -e "\033[31m[error] smqueue have been killed......\033[0m"
else
    echo -e "\033[32m[info] smqueue pid: $smqueue_pid is stopping......\033[0m"
    kill -9 $smqueue_pid
    echo -e "\033[32m[success] smqueue close complete......\033[0m"
fi

# Stop sipauthserve if pid is not null.
if [ ! -n "$sipauthserve_pid" ] 
then
    echo -e "\033[31m[error] sipauthserve have been killed......\033[0m"
else
    echo -e "\033[32m[info] sipauthserve pid: $sipauthserve_pid is stopping......\033[0m"
    kill -9 $sipauthserve_pid
    echo -e "\033[32m[success] sipauthserve close complete......\033[0m"
fi

# Stop asterisk if pid is not null.
if [ ! -n "$asterisk_pid" ] 
then
    echo -e "\033[31m[error] asterisk have been killed......\033[0m"
else
    echo -e "\033[32m[info] asterisk pid: $asterisk_pid is stopping......\033[0m"
    asterisk -rx 'core stop now'
    echo -e "\033[32m[success] asterisk close complete......\033[0m"
fi

echo -e "\033[32m[info] start to stop OpenBTS......\033[0m"
# Stop OpenBTS

openbts_pid=$(ps -aux | grep -v 'grep'  | grep /OpenBTS/OpenBTS | awk '{print $2}')
if [ ! -n "$openbts_pid" ] 
then
    echo -e "\033[31m[error] OpenBTS is not running......\033[0m"
else
    echo -e "\033[32m[info] OpenBTS pid: $openbts_pid are stopping......\033[0m"
    kill -9 $openbts_pid   
    echo -e "\033[32m[success] OpenBTS close complete......\033[0m"
fi

echo -e "\033[32m[info] start to stop transceiver......\033[0m"
transceiver_pid=$(ps -aux | grep -v 'grep'  | grep transceiver | awk '{print $2}')
if [ ! -n "$transceiver_pid" ] 
then
    echo -e "\033[31m[error] transceiver have been killed......\033[0m"
else
    echo -e "\033[32m[info] start to kill transceiver pid: $transceiver_pid......\033[0m"
    kill -9 $transceiver_pid
    echo -e "\033[32m[success] transceiver close complete......\033[0m"
fi

# Check for the existence of OpenBTS.pid file.
echo -e "\033[32m[info]start to RM OpenBTS.pid......\033[0m"
OpenBTS_pis_file="/var/run/OpenBTS.pid"
if [ -e "$file_path" ]; then
    # If OpenBTS.pid exists, delete it.
    rm "$file_path"
    echo -e "\033[32m[info] Delete OpenBTS.pid......\033[0m"
else
    echo -e "\033[31m[error] OpenBTS.pid file doed not exist......\033[0m"
fi

# Init user config database
# replace /var/run/TMSITable.db
echo -e "\033[32m[info] start to replace TMSITable.db......\033[0m"
tmsitable_init="/OpenBTS/TMSITable_init.db"
tmsitable="/var/run/TMSITable.db"
cp "$tmsitable_init" "$tmsitable"
echo -e "\033[32m[success] finish replace sqlite3.db......\033[0m"

# replace /var/lib/asterisk/sqlite3dir/sqlite3.db
echo -e "\033[32m[info] start to replace sqlite3.db......\033[0m"
asterisk_sqlite3_init="/OpenBTS/sqlite3_init.db"
asterisk_sqlite3="/var/lib/asterisk/sqlite3dir/sqlite3.db"
cp "$asterisk_sqlite3_init" "$asterisk_sqlite3"
echo -e "\033[32m[success] finish replace sqlite3.db......\033[0m"

# stop finish
echo -e "\033[32m[info] stop finish......\033[0m"

