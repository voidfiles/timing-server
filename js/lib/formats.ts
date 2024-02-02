import { Channel } from "./channels";

const SPACE_ASCII = 0x20;
const getChar = (channel: Channel, offset: number) => {
  let ch = channel.data[offset];
  if (ch == SPACE_ASCII || ch == 63) {
    return "_";
  }
  return String.fromCharCode(ch);
};

export enum Kind {
  RAW = 0,
  CLOCK = 1,
  LANE = 2,
  EVENT = 3,
}

type ChannelFormatter = (channel: Channel) => string;

export interface Formatter {
  kind: Kind;
  format: ChannelFormatter;
}

class RawFormatter implements Formatter {
  kind: Kind = Kind.RAW;

  format(channel: Channel): string {
    return channel.data
      .map((v) => {
        return String.fromCharCode(v);
      })
      .join("");
  }
}

class ClockFormatter implements Formatter {
  kind: Kind = Kind.CLOCK;

  format(channel: Channel): string {
    if (getChar(channel, 5) == "0" && getChar(channel, 6) == "0") {
      return "--:--.-";
    } else {
      const values =
        getChar(channel, 2) +
        getChar(channel, 3) +
        ":" +
        getChar(channel, 4) +
        getChar(channel, 5) +
        "." +
        getChar(channel, 6) +
        getChar(channel, 7);
      return values.replaceAll("_", "0");
    }
  }
}

class LaneFormatter extends ClockFormatter {
  kind: Kind = Kind.LANE;

  format(channel: Channel): string {
    const time = super.format(channel);

    return getChar(channel, 0) + " " + getChar(channel, 1) + " " + time;
  }
}

class EventFormatter extends ClockFormatter {
  kind: Kind = Kind.EVENT;

  format(channel: Channel): string {
    const event =
      getChar(channel, 0) + getChar(channel, 1) + getChar(channel, 2);
    const heat =
      getChar(channel, 5) + getChar(channel, 6) + getChar(channel, 7);

    return (event + ", " + heat).replaceAll("_", "");
  }
}

export const formats: Formatter[] = [
  new RawFormatter(),
  new ClockFormatter(),
  new LaneFormatter(),
  new EventFormatter(),
];

export const formatForKind = (kind?: Kind): Formatter => {
  if (kind === undefined) {
    return formats[Kind.RAW];
  }

  return formats[kind];
};
