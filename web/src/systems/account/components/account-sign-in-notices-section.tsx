import { Button, ListingRow, Pill } from "@compozy/ui";

import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import { SettingsGroup } from "@/systems/settings/components/settings-group";

import type { AccountSignInNoticeView } from "../lib/account-view";

export interface AccountSignInNoticesSectionProps {
  notices: readonly AccountSignInNoticeView[];
  currentDeviceIsAdmin: boolean;
  isDismissing?: boolean;
  onRevokeNotice: (notice: AccountSignInNoticeView) => void;
  onDismiss: () => void;
}

/**
 * Settings → Account → New sign-in notices (`_uiux.md` U3, Rule 32, US-034 AC-3):
 * lists new devices signed in within the last 7 days with per-device Revoke and a
 * single Dismiss action scoped to this installation.
 */
export function AccountSignInNoticesSection({
  notices,
  currentDeviceIsAdmin,
  isDismissing = false,
  onRevokeNotice,
  onDismiss,
}: AccountSignInNoticesSectionProps) {
  if (notices.length === 0) return null;

  return (
    <SettingsGroup
      action={
        <Button
          disabled={isDismissing}
          onClick={onDismiss}
          size="sm"
          type="button"
          variant="outline"
        >
          Dismiss
        </Button>
      }
      data-testid="account-sign-in-notices"
      description="New devices signed in to your account in the last 7 days. Dismiss hides these notices on this computer only."
      title={
        <span className="inline-flex items-center gap-2">
          <span>New sign-ins</span>
          <Pill size="xs" tone="warning">
            {notices.length}
          </Pill>
        </span>
      }
    >
      {notices.map(notice => (
        <ListingRow
          className="min-h-14 py-3"
          data-notice-device-id={notice.deviceId}
          interactive={false}
          key={`${notice.deviceId}-${notice.at}`}
        >
          <ListingRow.Icon>
            <ComputerPlatformIcon deviceKind={notice.kind} size="sm" tone="muted" />
          </ListingRow.Icon>

          <ListingRow.Main>
            <ListingRow.Name>
              <ListingRow.Title>New sign-in on {notice.name}</ListingRow.Title>
            </ListingRow.Name>
            <ListingRow.Meta className="mt-1">
              {notice.location ? (
                <>
                  <span>{notice.location}</span>
                  <ListingRow.MetaDot />
                </>
              ) : null}
              <span>{notice.whenLabel}</span>
            </ListingRow.Meta>
          </ListingRow.Main>

          <ListingRow.Trail>
            <Button
              disabled={!currentDeviceIsAdmin}
              onClick={() => onRevokeNotice(notice)}
              size="sm"
              title={
                currentDeviceIsAdmin
                  ? `Revoke ${notice.name}`
                  : "Only a device trusted for admin can do this."
              }
              type="button"
              variant="outline"
            >
              Revoke this device
            </Button>
          </ListingRow.Trail>
        </ListingRow>
      ))}
    </SettingsGroup>
  );
}
