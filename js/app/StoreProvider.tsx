"use client";
import { useRef } from "react";
import { Provider } from "react-redux";
import { makeStore } from "../lib/store";
import { PersistGate } from "redux-persist/integration/react";

const Loading = (
  <html>
    <body>loading</body>
  </html>
);
export default function StoreProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  // Create the store instance the first time this renders
  const { store, persistor } = makeStore();

  return (
    <Provider store={store}>
      <PersistGate loading={Loading} persistor={persistor}>
        {children}
      </PersistGate>
    </Provider>
  );
}
