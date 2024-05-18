#!/bin/bash

flyctl ssh sftp get -a saws-world /app/saws_world_data/saws.sqlite ./sw_dump_$(date +%d_%m_%Y).sqlite
