// The emulator ships its layout stylesheet as a package file. It is imported
// dynamically alongside the emulator itself so it rides the same lazy chunk,
// and `noUncheckedSideEffectImports` needs the specifier declared to allow it.
declare module "@xterm/xterm/css/xterm.css";
