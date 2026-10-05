import { Call, Events } from "@wailsio/runtime";
export const api = <T>(method: string, ...args: unknown[]): Promise<T> =>
  Call.ByName("main.App." + method, ...args);
export const on = (name: string, fn: (data: unknown) => void) =>
  Events.On(name, (event) => fn(event.data));
