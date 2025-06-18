import machine
import micropython
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
    if cmd[0] == "sendimg":
        if len(cmd) < 2 or not cmd[1].isdigit():
            return
        size = int(cmd[1])
        if size > 1000000:
            print("FAIL, too big")
            return
        
        print(f"OK READY {size}")
        
        micropython.kbd_intr(-1) # Disable keyboard interrupt whilst sending binary
        
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
        
        tft.png(UPLOAD_IMAGE, 0, 0)

if __name__=='__main__':
    # Initialise the screen
    tft = tft_config.config(0, buffer_size=4096)
    tft.init()
    
    # Attempt to delete any unfinished transfers
    try:
        os.delete(UPLOAD_PARTIAL)
    except:
        pass
    
    # Attempt to load the user's image, but fallback if it fails
    try:
        tft.png(UPLOAD_IMAGE, 0, 0)
    except:
        tft.png(DEFAULT_IMAGE, 0, 0)

    # Input/Output loop
    while(True):
        read_input()
        update_slider_values()
        send_slider_values()
        utime.sleep(0.01)
