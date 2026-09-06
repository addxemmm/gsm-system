import os
import json
import sqlite3
import subprocess
from flask import Flask, request

app = Flask(__name__)

@app.route('/start', methods={"POST"})
def start():
    result = start_openbts()
    return result

@app.route('/stop', methods={"POST"})
def stop():
    result = stop_openbts()
    return result

@app.route('/config', methods=["POST"])
def config():
    conf = request.get_data()
    json_conf = json.loads(conf)
    # get configuration parameters
    id = json_conf.get("id")
    result = config_openbts(id)
    return result

@app.route('/getconfig', methods=["POST"])
def getconfig():
    result = getconfig_openbts()
    return result

@app.route('/allconfig', methods=["POST"])
def allconfig():
    conf = request.get_data()
    json_conf = json.loads(conf)
    name = json_conf.get("name")
    value = json_conf.get("value")
    result = allconfig_openbts(name, value)
    return result

@app.route("/iptables", methods=["POST"])
def iptables():
    iface = json.loads(request.get_data()).get("iface")
    result = conf_iptables(iface)
    return result

@app.route("/smsinfo", methods=["POST"])
def smsinfo():
    result = get_sms_info()
    return result

@app.route("/ueinfo", methods=["POST"])
def ueinfo():
    result = get_ue_info()
    return result

@app.route("/setphonenumber", methods=["POST"])
def setphonenumber():
    json_conf = json.loads(request.get_data())
    imsi = json_conf.get("imsi")
    phone_number = json_conf.get("number")
    result = set_phone_number(imsi, phone_number)
    return result

@app.route("/sendsms", methods=["POST"])
def sendsms():
    json_conf = json.loads(request.get_data())
    imsi = json_conf.get("imsi")
    sender = json_conf.get("sender")
    smsmessage = json_conf.get("smsmessage")
    result = send_smsmessage(imsi, sender, smsmessage)
    return result


########################################################
def start_openbts():
    ### default start failed
    status = False
    message_id = 0 # 0 -> start failed  1 -> start success  2 -> is running   3 -> device is not connected, please connect usrp device.
    message = "Start Failed"
    # Determine whether the device is connect
    ps_command_resault = os.popen("ps -aux | grep -v 'grep' | grep /OpenBTS/OpenBTS").read()
    if(len(ps_command_resault) == 0):
        os.popen("rm /var/run/OpenBTS.pid")
        # Determine whether usrp is connected.
        if(usrpConnect()):
            current_path = os.getcwd()
            run_path = current_path + "/run.sh"
            subprocess.call(["bash", run_path])
            # get pid
            openbts_pid = os.popen("ps -aux| grep -v 'grep' | grep /OpenBTS/OpenBTS").read()
            sipauthserve_pid = os.popen("ps -aux| grep -v 'grep' | grep sipauthserve").read()
            smqueue_pid = os.popen("ps -aux| grep -v 'grep' | grep smqueue").read()
            asterisk_pid = os.popen("ps -aux| grep -v 'grep' | grep asterisk").read()
            # Determine whether the program is started
            if(len(openbts_pid) != 0 and len(sipauthserve_pid) != 0 and len(smqueue_pid) != 0 and len(asterisk_pid) != 0):
                status = True
                message_id = 1
                message = "Start successfully"
        else:
            message_id = 3
            message = "device is not connected, please connect usrp device."
    else:
        message_id = 2
        message = "is running"

    result = {'status': status, "message_id": message_id, "message": message}
    result_json = json.dumps(result)
    print(result_json)
    return result_json

def stop_openbts():
    status = False
    message_id = 0 # 0 -> stop failed   1 -> stop success   2 -> not running
    message = "Stop failed."
    # get pid
    openbts_pid = os.popen("ps -aux| grep -v 'grep' | grep /OpenBTS/OpenBTS").read()
    sipauthserve_pid = os.popen("ps -aux| grep -v 'grep' | grep sipauthserve").read()
    smqueue_pid = os.popen("ps -aux| grep -v 'grep' | grep smqueue").read()
    asterisk_pid = os.popen("ps -aux| grep -v 'grep' | grep asterisk").read()
    transceiver_pid = os.popen("ps -aux| grep -v 'grep' | grep transceiver").read()

    if(len(openbts_pid) == 0 and len(sipauthserve_pid) == 0 and len(smqueue_pid) == 0 and len(asterisk_pid) == 0 and len(transceiver_pid) == 0):
        message_id = 2
        message = "Not running."
    else:
        current_path = os.getcwd()
        #stop_path = current_path + "/stop.sh"
        #subprocess.call(["bash", stop_path])
        stop()
        os.system("sleep 3")

        # get pid
        openbts_pid = os.popen("ps -aux| grep -v 'grep' | grep /OpenBTS/OpenBTS").read()
        sipauthserve_pid = os.popen("ps -aux| grep -v 'grep' | grep sipauthserve").read()
        smqueue_pid = os.popen("ps -aux| grep -v 'grep' | grep smqueue").read()
        asterisk_pid = os.popen("ps -aux| grep -v 'grep' | grep asterisk").read()
        transceiver_pid = os.popen("ps -aux| grep -v 'grep' | grep transceiver").read()
        
        if(len(openbts_pid) == 0 and len(sipauthserve_pid) == 0 and len(smqueue_pid) == 0 and len(asterisk_pid) == 0 and len(transceiver_pid) == 0):
            status = True
            message_id = 1
            message = "Stop successfully."
    result = {'status': status, "message_id": message_id, "message": message}
    result_json = json.dumps(result)
    print(result_json)
    return result_json

def stop():
    smqueue_pid = os.popen("ps -aux | grep -v 'grep'  | grep smqueue | awk '{print $2}'").read()
    asterisk_pid = os.popen("ps -aux | grep -v 'grep'  | grep asterisk | awk '{print $2}'").read()
    sipauthserve_pid = os.popen("ps -aux | grep -v 'grep'  | grep sipauthserve | awk '{print $2}'").read()
    transceiver_pid = os.popen("ps -aux | grep -v 'grep'  | grep transceiver | awk '{print $2}'").read()
    if(len(smqueue_pid) == 0):
        print("smqueue has been killed......")
    else:
        print("smqueue pid: " + smqueue_pid + " is stopping......")
        os.popen("kill -9 " + smqueue_pid)
        print("smqueue close complete......")

    if(len(asterisk_pid) == 0):
        print("asterisk has been killed......")
    else:
        print("asterisk pid: " + asterisk_pid + " is stopping......")
        os.popen("kill -9 " + asterisk_pid)
        print("asterisk close complete......")

    if(len(sipauthserve_pid) == 0):
        print("sipauthserve has been killed......")
    else:
        print("sipauthserve pid: " + sipauthserve_pid + " is stopping......")
        os.popen("kill -9 " + sipauthserve_pid)
        print("sipauthserve close complete......")

    os.popen("rm /var/run/OpenBTS.pid")
    openbts_pid = os.popen("ps -aux | grep -v 'grep'  | grep /OpenBTS/OpenBTS | awk '{print $2}'").read()
    if(len(openbts_pid) == 0):
        print("openbts has been killed......")
    else:
        print("openbts pid: " + openbts_pid + " is stopping......")
        os.popen("/OpenBTS/OpenBTSCLI -c tmsis clear")
        os.popen("kill -9 " + openbts_pid)
        print("openbts close complete......")

    if(len(transceiver_pid) == 0):
        print("transceiver has been killed......")
    else:
        print("transceiver pid: " + transceiver_pid + " is stopping......")
        os.popen("kill -9 " + transceiver_pid)
        print("transceiver close complete......")

def config_openbts(id):
    status = False
    message_id = 0 # 0 -> Failed   1 -> SSuccess, please start system manually.uccess   2 -> Stop failed, please stop manually.    3 -> Can not find database file.
    message = "Failed."
    # 0 -> test 1 -> 
    # arfcns c0 band mcc mnc lac ci shortname
    config = [[1, "540", "1800", "001", "01", "4420", "41240", "test"], \
        [1, "55", "900", "460", "00", "4420", "41240", "ChinaMobile"], \
        [1, "540", "1800", "460", "00", "1", "0", "ChinaMobile"], \
        [1, "100", "900", "460", "01", "4420", "41240", "ChinaUnicom"], \
        [1, "668", "1800", "460", "01", "46980", "42186", "ChinaUnicom"]]

    if (os.path.exists("/etc/OpenBTS/OpenBTS.db")):
        openbts_pid = os.popen("ps -aux| grep -v 'grep' | grep /OpenBTS/OpenBTS").read()
        sipauthserve_pid = os.popen("ps -aux| grep -v 'grep' | grep sipauthserve").read()
        smqueue_pid = os.popen("ps -aux| grep -v 'grep' | grep smqueue").read()
        asterisk_pid = os.popen("ps -aux| grep -v 'grep' | grep asterisk").read()
        transceiver_pid = os.popen("ps -aux| grep -v 'grep' | grep transceiver").read()
    
        # stop system first
        stop_result = json.loads(stop_openbts())
        stop_message_id = stop_result.get("message_id")
        if (stop_message_id == 0):
            print("Stop failed, please stop manually.")
            status = False
            message_id = 2
            message = "Stop failed, please stop manually."
        else:
            # connect to sqlite database
            openbts_conn = sqlite3.connect('/etc/OpenBTS/OpenBTS.db')
            cursor = openbts_conn.cursor()
            # config
            cursor.execute("UPDATE CONFIG SET VALUESTRING='" + config[id][0] +"' WHERE KEYSTRING='GSM.Radio.ARFCNs';")
            cursor.execute("UPDATE CONFIG SET VALUESTRING='" + config[id][1] +"' where KEYSTRING='GSM.Radio.C0';")
            cursor.execute("UPDATE CONFIG SET VALUESTRING='" + config[id][2] +"' where KEYSTRING='GSM.Radio.Band';")
            cursor.execute("UPDATE CONFIG SET VALUESTRING='" + config[id][3] +"' where KEYSTRING='GSM.Identity.MCC';")
            cursor.execute("UPDATE CONFIG SET VALUESTRING='" + config[id][4] +"' where KEYSTRING='GSM.Identity.MNC';")
            cursor.execute("UPDATE CONFIG SET VALUESTRING='" + config[id][5] +"' where KEYSTRING='GSM.Identity.LAC';")
            cursor.execute("UPDATE CONFIG SET VALUESTRING='" + config[id][6] +"' where KEYSTRING='GSM.Identity.CI';")
            cursor.execute("UPDATE CONFIG SET VALUESTRING='" + config[id][7] +"' where KEYSTRING='GSM.Identity.ShortName';")
            # close database connect
            openbts_conn.commit()
            openbts_conn.close()
            print("Modify the configuration file successfully.")
            status = True
            message_id = 1
            message = "Success, please start system manually."
    else:
        print("Can not open database file, please check the file: /etc/OpenBTS/OpenBTS.db")
        message_id = 3
        message = "Can not find database file."
    
    result = {'status': status, "message_id": message_id, "message": message}
    result_json = json.dumps(result)
    print(result_json)
    return result_json

def getconfig_openbts():
    status = False
    message_id = 0 # 0 -> Failed   1 -> Success    3 -> Can not find database file.
    message = "Failed."
    data = None
    
    if (os.path.exists("/etc/OpenBTS/OpenBTS.db")):
        # connect to sqlite database
        openbts_conn = sqlite3.connect('/etc/OpenBTS/OpenBTS.db')
        cursor = openbts_conn.cursor()
        cursor.execute("SELECT KEYSTRING,VALUESTRING FROM CONFIG;")
        data = cursor.fetchall()
        status = True
        message_id = 1
        message = "Success"
        openbts_conn.commit()
        openbts_conn.close()

    else:
        print("Can not open database file, please check the file: /etc/OpenBTS/OpenBTS.db")
        message_id = 3
        message = "Can not find database file."
  
    result = {'status': status, "message_id": message_id, "message": message, "data": data}
    result_json = json.dumps(result)
    print(result_json)
    return result_json

def allconfig_openbts(name, value):
    status = False
    message_id = 0 # 0 -> Failed   1 -> Success    2 -> Can not find database file.
    message = "Failed."
    data = None
    
    if (os.path.exists("/etc/OpenBTS/OpenBTS.db")):
        # stop system first
        stop_result = json.loads(stop_openbts())
        stop_message_id = stop_result.get("message_id")
        if (stop_message_id == 0):
            print("Stop failed, please stop manually.")
            status = False
            message_id = 2
            message = "Stop failed, please stop manually."
        else:
        # config
            # connect to sqlite database
            openbts_conn = sqlite3.connect('/etc/OpenBTS/OpenBTS.db')
            cursor = openbts_conn.cursor()
            cursor.execute("UPDATE CONFIG SET VALUESTRING='" + value + "' WHERE KEYSTRING='" + name + "';")
            status = True
            message_id = 1
            message = "Success"
            openbts_conn.commit()
            openbts_conn.close()

    else:
        print("Can not open database file, please check the file: /etc/OpenBTS/OpenBTS.db")
        message_id = 2
        message = "Can not find database file."
  
    result = {'status': status, "message_id": message_id, "message": message}
    result_json = json.dumps(result)
    print(result_json)
    return result_json

def usrpConnect():
    status = False # default not connect
    devices_status = os.popen("uhd_find_devices 2>&1 | grep B210").read()
    if(len(devices_status) > 0):
        status = True
    return status

def conf_iptables(iface):
    status = False
    message_id = 0
    message = "Failed" # 0 -> Failed  1 -> Success
    # Write iptables.rule
    iptables_file = "/OpenBTS/iptables.rules"
    fo = open(iptables_file, "w")
    fo.write("# Generated by iptables-save v1.4.4\n*nat\n:PREROUTING ACCEPT [0:0]\n:POSTROUTING ACCEPT [0:0]\n:OUTPUT ACCEPT [0:0]\n-A POSTROUTING -o " + iface + " -j MASQUERADE\nCOMMIT\n# Generated by iptables-save v1.4.4\n*filter\n:INPUT ACCEPT [0:0]\n:FORWARD ACCEPT [0:0]\n:OUTPUT ACCEPT [0:0]\nCOMMIT\n")
    fo.close()
    # iptables
    os.popen("iptables -t nat -A POSTROUTING -s 192.168.99.0/24 -o " + iface + " -j MASQUERADE")
    result = os.popen("iptables-restore < " + iptables_file).read()
    if len(result) == 0:
        status = True
        message_id = 1
        message = "Success"
    result = {'status': status, "message_id": message_id, "message": message}
    result_json = json.dumps(result)
    print(result_json)
    return result_json   

def get_sms_info():
    status = False
    message_id = 0 
    message = "Failed"   # 0 -> Failed 1 -> Success.   2 -> Can not find /var/log/syslog 3 -> SMS message not fount.
    sms_infos = []
    if (os.path.exists("/var/log/smqueue.log")):
        # search message
        message_grep = os.popen("cat /var/log/smqueue.log | grep smqueue.h:505:get_text:").read()
        message_info_grep = os.popen("cat /var/log/smqueue.log | grep -A 13 'Deliver message:'").read()
        if(len(message_grep) != 0 and len(message_info_grep) != 0):
            # use --\n to splite message
            message_grep_groups = message_grep.split("\n")
            message_info_grep_groups = message_info_grep.split("--\n")
            # every 2 group are merged into one
            message_groups_num = int(len(message_grep_groups)/2)
            message_groups = []
            for i in range(message_groups_num):
                message_groups.append(message_grep_groups[2*i] + "\n" + message_info_grep_groups[i] )
            sms_infos = []
            for sms_message in message_groups:
                message_lines = sms_message.split("\n")
                sms_info = []

                # get time
                sms_time = message_lines[0].split()[2]
                sms_info.append(sms_time)

                # get sms
                sms_site = message_lines[0].find("Decoded text")
                if(sms_site >= 0):
                    sms = message_lines[0][sms_site+14:]
                else:
                    sms = None
                sms_info.append(sms)

   
                if_not_exist = sms_message.find("Can't send your SMS to")
                # can not fount reciver
                if(if_not_exist >= 0):
                    # get sender number
                    sender_number_from_site = sms_message.find("To: ")  
                    if(sender_number_from_site >= 0):
                        sender_number = message_lines[5].split("@")[0][9:]
                    else:
                        sender_number = None
                    sms_info.append(sender_number)

                    # get sender imsi
                    sender_imsi_site = sms_message.find("MESSAGE sip:IMSI")
                    if(sender_imsi_site >= 0):
                        while(sms_message[sender_imsi_site+16:sender_imsi_site+17] == " "):
                            sender_imsi_site = sms_message.find("MESSAGE sip:IMSI", sender_imsi_site+16)            
                        sender_imsi = sms_message[sender_imsi_site+16:sender_imsi_site+16+15]
                    else:
                        sender_imsi = None
                    sms_info.append(sender_imsi)

                    # get reciver number
                    reciver_number = message_lines[11].split()[5][:-1]
                    sms_info.append(reciver_number)

                    # get reciver imsi         
                    reciver_imsi = None
                    sms_info.append(reciver_imsi)
                    



                else:
                    # get sender number
                    sender_number_from_site = sms_message.find("From: ")  
                    if(sender_number_from_site >= 0):
                        sender_number = message_lines[5].split()[1]
                    else:
                        sender_number = None
                    sms_info.append(sender_number)

                    # get sender imsi
                    sender_imsi_site = sms_message.find("Contact: <sip:IMSI")  
                    if(sender_imsi_site >= 0):
                        while(sms_message[sender_imsi_site+18:sender_imsi_site+19] == " "):
                            sender_imsi_site = sms_message.find("Contact: <sip:IMSI", sender_imsi_site+18)
                        sender_imsi = sms_message[sender_imsi_site+18:sender_imsi_site+18+15]
                    else:
                        sender_imsi = None
                    sms_info.append(sender_imsi)
                    
                    # get reciver number
                    reciver_number_site = sms_message.find("To: ")
                    if(reciver_number_site >= 0):
                        reciver_number = message_lines[6].split()[1]
                    else:
                        reciver_number = None
                    sms_info.append(reciver_number)

                    # get reciver imsi
                    reciver_imsi_site = sms_message.find("MESSAGE sip:IMSI")
                    if(reciver_imsi_site >= 0):
                        while(sms_message[reciver_imsi_site+16:reciver_imsi_site+17] == " "):
                            reciver_imsi_site = sms_message.find("MESSAGE sip:IMSI", reciver_imsi_site+16)            
                        reciver_imsi = sms_message[reciver_imsi_site+16:reciver_imsi_site+16+15]
                    else:
                        reciver_imsi = None
                    sms_info.append(reciver_imsi)

                sms_infos.append(sms_info)
            status = True
            message_id = 1
            message = "Success"
        else:
            message_id = 3
            message = "SMS message not fount."
    else:
        message_id = 2
        message = "Can not find /var/log/syslog"

    result = {'status': status, "message_id": message_id, "message": message, "infos": sms_infos}
    result_json = json.dumps(result)
    print(result_json)
    return result_json 

def get_ue_info():
    ### default getinfo failed
    status = False
    message_id = 0 # 0 -> Failed  1 -> Success 2 -> System is not running.  3 -> Can not find info.
    message = "Start Failed"
    ue_infos = []
    # get pid
    openbts_pid = os.popen("ps -aux| grep -v 'grep' | grep /OpenBTS/OpenBTS").read()
    if(len(openbts_pid) == 0):
        message_id = 2
        message = "System is not running."
    else:
        # get sgsn info
        sgsn_info_results = os.popen("/OpenBTS/OpenBTSCLI -c sgsn list").read().strip()
        sgsn_info_results_list = sgsn_info_results.split("\n")
        sgsn_info_results_list = [x for x in sgsn_info_results_list if x != "" ]
        sgsn_infos = []
        if(sgsn_info_results_list is not None):
            for sgsn_info_result in sgsn_info_results_list:
                sgsn_info = []
                sgsn_info_list = sgsn_info_result.split()
                sgsn_info.append(sgsn_info_list[2][5:])
                sgsn_info.append(sgsn_info_list[9][4:])
                sgsn_infos.append(sgsn_info)
        
        # get tmsis info
        tmsis_info_results = os.popen("/OpenBTS/OpenBTSCLI -c tmsis -l").read().strip()
        tmsis_info_results_list = tmsis_info_results.split("\n")
        if(len(tmsis_info_results_list[1:]) > 0):
            for tmsis_info_result in tmsis_info_results_list[1:]:
                ue_info = []
                tmsis_info_list = tmsis_info_result.split()
                tmsis_info_imsi = tmsis_info_list[0]
                tmsis_info_imei = tmsis_info_list[2]
                tmsis_info_number = tmsis_info_list[10][5:-1]
                ue_info.append(tmsis_info_imsi)
                ue_info.append(tmsis_info_imei)
                ue_info.append(tmsis_info_number)
                ue_info_ips = None
                for sgsn_info in sgsn_infos:
                    if (tmsis_info_imsi == sgsn_info[0]):
                        ue_info_ips = sgsn_info[1]        
                ue_info.append(ue_info_ips)
                ue_infos.append(ue_info)
            status = True
            message_id = 1
            message = "Success"
        else:
            message_id = 3
            message = "Can not find info."
    result = {'status': status, "message_id": message_id, "message": message, "infos": ue_infos}
    result_json = json.dumps(result)
    print(result_json)
    return result_json   

def set_phone_number(imsi, phone_number):
    status = False
    message_id = 0 
    message = "False"   
    # 0 -> False, please check /var/log/syslog. 
    # 1 -> Success.   
    # 2 -> Can not find /var/run/TMSITable.db or /val/lib/asterisk/sqlite3dir/sqlite3.db  
    # 3 -> This imsi is not existed.
    # 4 -> This imsi is not existed in asterisk.

    tmsi_table_file = "/var/run/TMSITable.db"
    asterisk_sqlite3_file = "/var/lib/asterisk/sqlite3dir/sqlite3.db"

    if (os.path.exists(tmsi_table_file) and os.path.exists(asterisk_sqlite3_file)):
        # connect to tmsi_table database
        tmsi_table_conn = sqlite3.connect(tmsi_table_file)
        tmsi_cursor = tmsi_table_conn.cursor()
        # connect to asterisk database
        asterisk_conn = sqlite3.connect(asterisk_sqlite3_file)
        asterisk_cursor = asterisk_conn.cursor()      
        tmsi_cursor.execute("select * from tmsi_table where IMSI='" + imsi + "';")
        asterisk_cursor.execute("select * from sip_buddies where name='IMSI" + imsi + "';")
        if (len(tmsi_cursor.fetchall()) != 0):
            if(len(asterisk_cursor.fetchall()) != 0):
                tmsi_cursor.execute("update tmsi_table set ASSOCIATED_URI='<tel:" + phone_number + ">' where IMSI='" + imsi + "';")
                asterisk_cursor.execute("update sip_buddies set callerid='" + phone_number + "' where name='IMSI" + imsi + "';")
                asterisk_cursor.execute("update dialdata_table set exten='" + phone_number + "' where dial='IMSI" + imsi + "';")
                status = True
                message_id = 1
                message = "Success"
            else:
                tmsi_cursor.execute("update tmsi_table set ASSOCIATED_URI='<tel:" + phone_number + ">' where IMSI='" + imsi + "';")
                message_id = 4
                message = "This imsi is not existed in asterisk."
        else:
            message_id = 3
            message = "This imsi is not existed."
        # close database connect
        tmsi_table_conn.commit()
        tmsi_table_conn.close()
        asterisk_conn.commit()
        asterisk_conn.close()
    else:
        message_id = 2
        message = "Can not find /var/run/TMSITable.db or /val/lib/asterisk/sqlite3dir/sqlite3.db"

    result = {'status': status, "message_id": message_id, "message": message}
    result_json = json.dumps(result)
    print(result_json)
    return result_json  

def send_smsmessage(imsi, sender, smsmessage):
    status = False
    message_id = 0 # 0 -> send failed   1 -> send success   2 -> not running
    message = "Send failed."
    # get pid
    openbts_pid = os.popen("ps -aux| grep -v 'grep' | grep /OpenBTS/OpenBTS").read()
    sipauthserve_pid = os.popen("ps -aux| grep -v 'grep' | grep sipauthserve").read()
    smqueue_pid = os.popen("ps -aux| grep -v 'grep' | grep smqueue").read()
    asterisk_pid = os.popen("ps -aux| grep -v 'grep' | grep asterisk").read()
    transceiver_pid = os.popen("ps -aux| grep -v 'grep' | grep transceiver").read()

    if(len(openbts_pid) == 0 and len(sipauthserve_pid) == 0 and len(smqueue_pid) == 0 and len(asterisk_pid) == 0 and len(transceiver_pid) == 0):
        message_id = 2
        message = "Not running."
    else:
        send_command = "/OpenBTS/OpenBTSCLI -c sendsms " + str(imsi) + " " + str(sender) + " " + str(smsmessage)
        print(send_command)
        send_result = os.popen(send_command).read()
        if("message submitted for delivery" in send_result):
            status = True
            message_id = 1
            message = "Send successfully."
    result = {'status': status, "message_id": message_id, "message": message}
    result_json = json.dumps(result)
    print(result_json)
    return result_json