import _thread
import machine
import micropython
import gc
import select
import sys
import os
import tft_config
import utime

# Deej consts
NUM_SLIDERS = 2
NUM_SWITCHES = 3

analogSliderValues = [0, 0]
switchValues = [
    machine.Pin(2, machine.Pin.IN, machine.Pin.PULL_UP),
    machine.Pin(3, machine.Pin.IN, machine.Pin.PULL_UP),
    machine.Pin(4, machine.Pin.IN, machine.Pin.PULL_UP)
]

slideMaster = machine.ADC(26)
slideProg1 = machine.ADC(27)

# New polling object for stdin
serial = select.poll() 
serial.register(sys.stdin, select.POLLIN) 

# Image consts
DEFAULT_IMAGE = "default.png"
UPLOAD_IMAGE = "uploaded.dat"
UPLOAD_PARTIAL = "uploaded.part"

# Animated Image consts
ANIM_FOLDER = "anim"
MAX_FRAMES = 120

MAX_FILE_SIZE = 1000000

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
    if not serial.poll(0):
        return
    
    res = ""
    while serial.poll(0):
        res+=(sys.stdin.read(1))
    cmd = res.split(" ")
    if cmd[0] == "sendstaticimg":
        if len(cmd) < 2 or not cmd[1].isdigit():
            print("FAIL, invalid arguments")
            return
        
        size = int(cmd[1])
        if size > MAX_FILE_SIZE:
            print("FAIL, too big")
            return
        
        print(f"OK READY {size}")
        
        micropython.kbd_intr(-1) # Disable keyboard interrupt whilst sending binary
        
        gc.collect()
        
        total_recv = 0
        with open(UPLOAD_PARTIAL, "wb") as f:
            while total_recv < size:
                if serial.poll(0):
                    remaining = size - total_recv
                    
                    chunk = min(1024, remaining)
                    f.write(sys.stdin.buffer.read(chunk))
                    
                    total_recv += chunk
                    print(f"OK {total_recv}")
        
        micropython.kbd_intr(0x03) # Re-enable Keyboard interrupt
        
        print(f"OK DONE {total_recv}")

        os.remove(UPLOAD_IMAGE)
        os.rename(UPLOAD_PARTIAL, UPLOAD_IMAGE)
        os.sync()
        
        gc.collect()
        
        tft.png(UPLOAD_IMAGE, 0, 0)
    elif cmd[0] == "sendanimated":
        if len(cmd) < 2 or not cmd[1].isdigit():
            print("FAIL, invalid arguments")
            return
        
        nFrames = int(cmd[1])
        
        if nFrames > MAX_FRAMES:
            print("FAIL, too many frames")
            return
        
        eFiles = None
        try:
            eFiles = os.listdir(ANIM_FOLDER)
        except:
            pass
        
        if eFiles == None:
            try:
                os.mkdir(ANIM_FOLDER)
            except:
                print("FAIL, cannot create folder for animations")
                return
        
        try:
            for file in eFiles:
                os.remove(f"{ANIM_FOLDER}/{file}")
        except:
            print("FAIL, cannot clear old animation directory")
            return
        
        print(f"OK FRAMES {nFrames}")
        
        for nF in range(nFrames):
            total_recv = 0
            rawSize = sys.stdin.buffer.readline().split(" ")
            if len(rawSize) < 2 or rawSize[1].isdigit():
                print("FAIL, frame size incorrectly specified")
                return
            
            size = int(rawSize[1])
            
            print(f"OK READY {size}")
           
            micropython.kbd_intr(-1) # Disable keyboard interrupt whilst sending binary
           
            with open(UPLOAD_PARTIAL, "wb") as f:
                while total_recv < size:
                    if serial.poll(0):
                        remaining = size - total_recv
                        
                        chunk = min(1024, remaining)
                        f.write(sys.stdin.buffer.read(chunk))
                        
                        total_recv += chunk
                        print(f"OK {total_recv}")
            
            os.rename(UPLOAD_PARTIAL, f"{ANIM_FOLDER}/frame_{nF}.png")
            
            micropython.kbd_intr(0x03)
                        
        print(f"OK DONE FRAMES {nFrames}")

def do_animation():
    global tft
    
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
        while(True):
            for frame in frames:
                tft.png(f"{ANIM_FOLDER}/{frame}", 0, 0)

def do_sensor_loop():
    # Input/Output loop
    while(True):
        read_input()
        update_slider_values()
        send_slider_values()
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
