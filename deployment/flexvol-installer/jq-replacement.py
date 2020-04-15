#!/usr/bin/python
import sys
import json

data = json.load(sys.stdin)
findkey = sys.argv[1]
if findkey in data:
    print(data[findkey])
