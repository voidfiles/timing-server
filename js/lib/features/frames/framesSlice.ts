import { createSlice } from "@reduxjs/toolkit";
import type { PayloadAction } from "@reduxjs/toolkit";
import type { RootState } from "../../store";
import type { Channel } from "@/lib/channels";
import type { Kind } from "@/lib/formats";

// Define a type for the slice state
export interface FrameState {
  value: number;
  channels: Channel[];
  formats: ChannelFormat;
}

export type Channels = {
  [key: number]: Channel;
};

export type ChannelFormat = {
  [key: number]: Kind;
};

export interface FrameUpdate {
  value: number;
  channels: Channels;
}

export interface ChannelFormatUpdate {
  channel: number;
  format: Kind;
}

// Define the initial state using that type
const initialState: FrameState = {
  channels: [],
  value: 0,
  formats: {},
};

export const frameSlice = createSlice({
  name: "frame",
  // `createSlice` will infer the state type from the `initialState` argument
  initialState,
  reducers: {
    updateFormat: (state, action: PayloadAction<ChannelFormatUpdate>) => {
      state.formats[action.payload.channel] = action.payload.format;
    },
    updateFrame: (state, action: PayloadAction<FrameUpdate>) => {
      if (state.value != action.payload.value) {
        state.value = action.payload.value;
      }
      Object.entries(action.payload.channels).forEach(([k, channel], _) => {
        const i = channel.number || 0;
        channel.data.forEach((dataVal, n) => {
          const formatVal = channel.format[n];
          if (!state.channels[i]) {
            state.channels.splice(i, 0, {
              number: i,
              data: [0, 0, 0, 0, 0, 0, 0, 0],
              format: [0, 0, 0, 0, 0, 0, 0, 0],
            });
          }
          if (state.channels[i].data[n] != dataVal) {
            state.channels[i].data[n] = dataVal;
          }
          if (state.channels[i].format[n] != formatVal) {
            state.channels[i].format[n] = formatVal;
          }
        });
      });
    },
  },
});

export const { updateFrame, updateFormat } = frameSlice.actions;

// Other code such as selectors can use the imported `RootState` type
export const selectFrame = (state: RootState) => state.frame;

export default frameSlice.reducer;
