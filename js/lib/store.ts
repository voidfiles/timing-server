import { configureStore } from "@reduxjs/toolkit";
import frameSliceReducer from "./features/frames/framesSlice";
import Socket from "./socket";
import socketMiddleware from "./socketMiddleware";
import { persistStore, persistReducer } from "redux-persist";
import storage from "redux-persist/lib/storage"; // defaults to localStorage for web

const persistConfig = {
  key: "root",
  storage,
  blacklist: ["persist", "register"],
};

const frameSlicePersistReducer = persistReducer(
  persistConfig,
  frameSliceReducer
);

export const makeStoreWithoutPersistor = () => {
  return configureStore({
    reducer: {
      frame: frameSlicePersistReducer,
    },
    middleware: (getDefaultMiddleware) => {
      const sm = socketMiddleware(new Socket("ws://localhost:8000/ws"));
      const middle = getDefaultMiddleware().concat([sm]);
      console.log("Making a store", middle);
      return middle;
    },
  });
};

export const makeStore = () => {
  const store = makeStoreWithoutPersistor();

  const persistor = persistStore(store);

  return { store, persistor };
};

// Infer the type of makeStore
export type AppPersistor = ReturnType<typeof persistStore>;
export type AppStore = ReturnType<typeof makeStoreWithoutPersistor>;
// Infer the `RootState` and `AppDispatch` types from the store itself
export type RootState = ReturnType<AppStore["getState"]>;
export type AppDispatch = AppStore["dispatch"];
