#!/bin/bash
systemctl stop heatpump
chmod +x bin/ARM/heatpump
cp bin/ARM/heatpump /usr/bin
systemctl start heatpump
