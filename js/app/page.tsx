"use client";

import { selectFrame } from "@/lib/features/frames/framesSlice";
import { RootState } from "@/lib/store";
import { useEffect } from "react";
import { useDispatch, useSelector } from "react-redux";
import { startListening, stopListening } from "@/lib/socketMiddleware";
import { Channel } from "@/lib/channels";
import { Kind, formatForKind } from "@/lib/formats";
import { updateFormat } from "@/lib/features/frames/framesSlice";
const SelectFormat = ({
  channelId,
  channel,
  format,
}: {
  channelId: number;
  channel: Channel;
  format: Kind;
}): React.ReactElement => {
  const dispatch = useDispatch();

  return (
    <select
      name="pets"
      id="pet-select"
      defaultValue={Kind.RAW}
      value={format}
      onChange={(e) => {
        const f: Kind = parseInt(e.target.value, 10);

        dispatch(
          updateFormat({
            channel: channelId,
            format: f,
          })
        );
      }}
    >
      {Object.entries(Kind).map(([k, s], i) => {
        return <option value={k}>{s}</option>;
      })}
    </select>
  );
};

export default () => {
  const frame = useSelector((state: RootState) => selectFrame(state));
  const dispatch = useDispatch();
  useEffect(() => {
    dispatch(startListening());

    return () => {
      dispatch(stopListening());
    };
  }, []);

  const keys = Object.keys(frame.channels)
    .map((input: string) => {
      return parseInt(input, 10);
    })
    .sort((a, b) => {
      return a - b;
    });

  const formatForChannel = (channelId: number, channel: Channel): string => {
    const f = frame.formats[channelId];

    const formatter = formatForKind(f);

    return formatter.format(channel);
  };

  const selectForChannel = (
    channelId: number,
    channel: Channel
  ): React.ReactElement => {
    const format = frame.formats[channelId];

    return SelectFormat({ channelId, channel, format });
  };

  return (
    <table className="font-mono">
      <thead>
        <tr>
          <td>ch</td>
          <td>Kind</td>
          <td>Wire</td>
          <td>Data</td>
          <td>Raw</td>
          <td>Ints</td>
        </tr>
      </thead>
      <tbody>
        {keys.map((i: number) => {
          const channel = frame.channels[i];
          let data = formatForChannel(i, channel);
          const ints = channel.data.map((v, i) => {
            return (
              <span
                key={i}
                className="w-6 mr-2 text-center inline-block border-b border-black"
              >
                {v}
              </span>
            );
          });
          const raw = channel.data.map((v, i) => {
            let d = String.fromCharCode(v);
            if (d === " ") {
              d = "-";
            }
            return (
              <span
                key={i}
                className="w-6 mr-2 text-center inline-block border-b border-black"
              >
                {d}
              </span>
            );
          });
          const format = channel.format.map((v, i) => {
            let d = String.fromCharCode(v);
            if (d === " ") {
              d = "-";
            }
            return (
              <span
                key={i}
                className="w-6 mr-2 text-center inline-block border-b border-black"
              >
                {d}
              </span>
            );
          });

          return (
            <tr key={i}>
              <td>{i}</td>
              <td>{selectForChannel(i, channel)}</td>
              <td>{channel.preformatted}</td>
              <td>{data}</td>
              <td>{raw}</td>
              <td>{ints}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
};
