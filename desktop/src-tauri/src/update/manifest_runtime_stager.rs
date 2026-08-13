use semver::Version;
use url::Url;

use crate::home::CompozyHome;
use crate::runtime::artifacts::current_target;
use crate::runtime::provision::{ProvisionError, StagedRuntime, stage_update};

use super::runtime_update::{RuntimeStager, RuntimeUpdaterError};

pub struct ManifestRuntimeStager {
    home: CompozyHome,
    manifest_url: Url,
    public_key: String,
    previous_public_key: Option<String>,
    target: String,
}

impl ManifestRuntimeStager {
    pub fn new(
        home: CompozyHome,
        manifest_url: Url,
        public_key: impl Into<String>,
        previous_public_key: Option<String>,
    ) -> Result<Self, RuntimeUpdaterError> {
        Ok(Self {
            home,
            manifest_url,
            public_key: public_key.into(),
            previous_public_key,
            target: current_target()
                .map_err(|_| RuntimeUpdaterError::Version)?
                .to_owned(),
        })
    }
}

impl RuntimeStager for ManifestRuntimeStager {
    fn stage(&self, installed: &Version) -> Result<Option<StagedRuntime>, ProvisionError> {
        stage_update(
            &self.home,
            &self.manifest_url,
            &self.public_key,
            self.previous_public_key.as_deref(),
            &self.target,
            installed,
        )
    }
}
