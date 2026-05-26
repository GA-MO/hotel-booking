// Thin wrappers around @mantine/notifications. Pages should import these
// instead of calling notifications.show directly so the look/feel stays
// consistent (color, position, autoClose timing). Replaces the previous
// SuccessBanner/ErrorBanner mount-then-clear-in-state pattern for any
// click-action feedback that doesn't need to stay on the page.
import { notifications } from "@mantine/notifications";

export function notifySuccess(message: string, title?: string) {
  notifications.show({
    color: "teal",
    title,
    message,
    autoClose: 3500,
    withBorder: true,
  });
}

export function notifyError(message: string, title?: string) {
  notifications.show({
    color: "red",
    title,
    message,
    autoClose: 5000,
    withBorder: true,
  });
}

export function notifyInfo(message: string, title?: string) {
  notifications.show({
    color: "blue",
    title,
    message,
    autoClose: 3500,
    withBorder: true,
  });
}
