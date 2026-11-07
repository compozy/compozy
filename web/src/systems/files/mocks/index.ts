import { sessionFilesHandlers, workspaceFilesHandlers } from "./files-handlers";

export {
  fileContentFixture,
  fileListingsFixture,
  filesRootFixture,
  sessionFilesHandlers,
  sessionFilesRootUnavailableHandlers,
  workspaceFilesHandlers,
} from "./files-handlers";

export const handlers = [...sessionFilesHandlers, ...workspaceFilesHandlers];
