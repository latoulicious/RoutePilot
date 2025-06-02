import { Elysia } from "elysia";
import { getConfig } from "./config";

const config = getConfig();
const app = new Elysia().get("/", () => "Hello Elysia").listen(config.port);

console.log(
  `🦊 Elysia is running at ${app.server?.hostname}:${app.server?.port}`
);
