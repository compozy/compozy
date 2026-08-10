#[cfg(not(windows))]
use std::fs::File;
#[cfg(windows)]
use std::fs::OpenOptions;
use std::path::Path;

#[cfg(windows)]
use std::os::windows::fs::OpenOptionsExt;

pub fn sync_directory(path: &Path) -> std::io::Result<()> {
    #[cfg(windows)]
    let directory = OpenOptions::new()
        .read(true)
        .custom_flags(0x0200_0000)
        .open(path)?;
    #[cfg(not(windows))]
    let directory = File::open(path)?;
    directory.sync_all()
}
