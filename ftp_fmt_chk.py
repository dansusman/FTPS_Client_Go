#!/usr/bin/env python

# Format checker for CS3700 FTP Client Project at Northeastern University
import os
import sys
import argparse 
import subprocess

def check_windows_line_endings(project_dir, file):
    f = try_open(project_dir + '/' + file)
    content = f.read()
    if content.count('\r\n') > 2:
        # Safe to assume that the file is windows format
        print('The ' + file + ' file might contain Windows-style line endings, try converting the file to Unix format using dos2unix')
        sys.exit()


def run_make(project_dir):
    make = subprocess.Popen(['make'], cwd=project_dir, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    make_out = make.communicate()[0]
    make_ret = make.returncode

    if make_out == b'make: *** No targets.  Stop.\n':
        make_ret = 0

    if make_ret != 0:
        print('Error during make. Error code ' + str(make_ret))
        print(make_out.decode())
        sys.exit()


def try_open(filename, perms='r'):
    try:
        f = open(filename, perms)
    except:
        print("Error: Unable to open", filename)
        sys.exit()
    return f

parser = argparse.ArgumentParser()
parser.add_argument("project_directory", help="Path to the directory containing your project, i.e. the directory containing README.md and your Makefile")
args = parser.parse_args()

files = os.listdir(args.project_directory)
project_dir = os.path.abspath(args.project_directory)
readme = 'README.md'
client = '3700ftp'

if readme in files:
    check_windows_line_endings(project_dir, readme)
else:
    print('The README.md file is missing, make sure you named the file correctly')
    sys.exit()

run_make(project_dir)

files = os.listdir(args.project_directory)

if client not in files:
    print('The ' + client + ' program is missing, make sure you named the file correctly')
    sys.exit()

print('Looks like you have all the required files, you are good to go!')