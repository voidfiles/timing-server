#! /bin/bash

socat -d -d pty,raw,echo=0 pty,raw,echo=0 &
export socat_pid=`echo $!`
sleep 2;
stty -f /dev/ttys017 -a &
export cat_pid=`echo $!`
echo meet.bin > /dev/ttys018
sleep 30;
kill -9 $socat_pid;
kill -9 $cat_pid;