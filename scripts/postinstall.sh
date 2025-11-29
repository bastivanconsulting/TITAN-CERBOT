#!/bin/sh
systemctl daemon-reload
systemctl enable titan-certbot.timer
systemctl start titan-certbot.timer
