import machine

# File Size
MAX_FILE_SIZE = 1000000

# Image consts
DEFAULT_IMAGE = "default.png"
UPLOAD_IMAGE = "uploaded.dat"
UPLOAD_PARTIAL = "uploaded.part"

# Animated Image consts
ANIM_FOLDER = "anim"
MAX_FRAMES = 240

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