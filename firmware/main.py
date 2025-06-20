import _thread
from proj_const import *
import image_download
import machine
import select
import sys
import os
import tft_config
import utime

COMMANDS = {
    "sendstaticimg": image_download.download_static_image,
    "sendanimatedimg": image_download.download_animated_set
}

# New polling object for stdin
serial = select.poll() 
serial.register(sys.stdin, select.POLLIN) 

# Thread flags
STOP_ANIMATION_THREAD = False
ANIMATION_THREAD_EXITED = True

# Read each slider's position
def update_slider_values():
    for x in range(NUM_SLIDERS):
        if(x == 0):
            analogSliderValues[0] = slideMaster.read_u16() //64
        elif(x == 1):
            analogSliderValues[1] = slideProg1.read_u16() //64

# Make a string with all slider values and print it
def send_slider_values():
    builtString = ""
    builtString2 = ""
    
    for x in range(NUM_SLIDERS):
        builtString +=  str(analogSliderValues[x])
        
        if(x < NUM_SLIDERS - 1):
            builtString += '|'
    
    for z in range(NUM_SWITCHES):
        builtString2 += str(1 - switchValues[z].value())
        
        if (z < NUM_SWITCHES - 1):
            builtString2 += '|'
    
    print(f"{builtString} {builtString2}")

# Read loop for commands
def read_input():
    global STOP_ANIMATION_THREAD
    global ANIMATION_THREAD_EXITED
    
    if not serial.poll(0):
        return
    
    res = ""
    while serial.poll(0):
        res+=(sys.stdin.read(1))
    cmd = res.strip().split(" ")
    
    if cmd[0] not in COMMANDS:
        print(f"FAIL, Unknown command {cmd[0]}")
        return
    
    STOP_ANIMATION_THREAD = True
    while(not ANIMATION_THREAD_EXITED):
        print("WAIT, Animation thread stopping...")
        utime.sleep_ms(100)
    
    COMMANDS[cmd[0]](cmd)
    
    utime.sleep_ms(100)
    second_thread = _thread.start_new_thread(do_animation, ())

def do_animation():
    global tft
    global STOP_ANIMATION_THREAD
    global ANIMATION_THREAD_EXITED
    
    frames = []
    try:
        frames = sorted(os.listdir(ANIM_FOLDER))
    except:
        pass
    
    if len(frames) == 0:
        # Attempt to load the user's image, but fallback if it fails
        try:
            tft.png(UPLOAD_IMAGE, 0, 0)
        except:
            tft.png(DEFAULT_IMAGE, 0, 0)
    else:
        ANIMATION_THREAD_EXITED = False
        while(not STOP_ANIMATION_THREAD):
            for frame in frames:
                tft.png(f"{ANIM_FOLDER}/{frame}", 0, 0)
                if STOP_ANIMATION_THREAD:
                    break
        ANIMATION_THREAD_EXITED = True

def do_sensor_loop():
    # Input/Output loop
    while(True):
        read_input()
        update_slider_values()
        # send_slider_values()
        utime.sleep_ms(100)
    
if __name__=='__main__':
    # Initialise the screen
    tft = tft_config.config(0, buffer_size=4096)
    tft.init()
    
    # Attempt to delete any unfinished transfers
    try:
        os.delete(UPLOAD_PARTIAL)
    except:
        pass
    
    second_thread = _thread.start_new_thread(do_animation, ())
    do_sensor_loop()
