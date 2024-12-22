import sys


def main():
    print("Enter input (Ctrl+D to end):")
    MAX_LINES = 256
    COUNTER = 0
    channel = 0
    fmt = 0
    try:
        while True:
            COUNTER += 1
            if COUNTER >= MAX_LINES:
                break
            # Read 1 byte from standard input
            byte = sys.stdin.buffer.read(1)
            if not byte:
                break
            num = int.from_bytes(byte)
            if num >= 127:
                potential_channel = (num >> 1) & 0x1F ^ 0x1F
                fmt = num & 1
                if channel != potential_channel:
                    channel = potential_channel
                    print("Control %s END FRAME\n" % (num))
                    print("Control %s START FRAME\n" % (num))

                continue

            # Convert to hexadecimal and print
            segmentNum = (num & 0xF0) >> 4
            segmentInt = ((num << 4) & 0xF0) >> 4
            segmentData = "_"
            if segmentInt == 32:
                segmentData = "s"
            else:
                segmentInt = segmentInt ^ 0x0F + 48
                segmentData = chr(segmentInt)
            print(
                "%2s %2s %2s %2s %3s %3s %2s"
                % (fmt, channel, segmentNum, byte.hex(), num, segmentInt, segmentData)
            )
    except KeyboardInterrupt:
        # Gracefully handle interruptions
        print("\nInput interrupted.")
    finally:
        print("\nDone.")


if __name__ == "__main__":
    main()
