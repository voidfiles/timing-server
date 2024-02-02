import { Middleware, isAnyOf } from "@reduxjs/toolkit";
import Socket from "./socket";
import { createAction } from "@reduxjs/toolkit";
import { RootState } from "./store";
import { FrameUpdate, updateFrame } from "./features/frames/framesSlice";
export const startListening = createAction("socket/connect");
export const stopListening = createAction("socket/disconnect");

const isStop = isAnyOf(stopListening);
const isStart = isAnyOf(startListening);

const socketMiddleware: any =
  (socket: Socket): Middleware<{}, RootState> =>
  ({ dispatch, getState }) =>
  (next) =>
  (action) => {
    if (isStart(action)) {
      socket.connect();

      socket.on("message", (evt: any) => {
        const data: FrameUpdate = JSON.parse(evt.data);
        dispatch(updateFrame(data));
      });

      socket.on("connect", (data: any) => {
        console.log("connected", data);
      });
    }

    if (isStop(action)) {
      socket.disconnect();
    }

    return next(action);
  };

export default socketMiddleware;
