/**
 * Blueprint: `tpl-settings` — Sectioned Settings (pcs-saas 01 §9)
 *
 * Domain: Workspace settings → "General" group (profile, notifications, danger zone).
 *
 * Catalog nodes composed here:
 *   settings navigation (secondary vertical list; Select on mobile) · org-page-header (title + description)
 *   org-sectioned-form as cards, each with ONE save model:
 *     - per-card  → "Workspace profile" (CardFooter "Save" enabled only when dirty, "Cancel" restores)
 *     - instant   → "Notifications" (Switch applies on change, inline "Saved" 2s, revert on failure)
 *   danger zone card (last) → org-destructive-confirm with mol-confirm-input
 *   org-banner (S6 pending propagation) · org-section-error (per-card S4) · mol-form-field · mol-skeleton
 *
 * Acceptance checks satisfied (search for the tag):
 *   AC-SET-1 Each card declares one save model; instant controls show "Saved"; explicit cards disable Save when clean.
 *   AC-SET-2 Danger zone is the last section; every destructive action requires confirmation naming the target.
 *   AC-SET-3 Leaving a dirty per-card form prompts.
 *
 * Split into a project:
 *   src/routes/_app/settings/route.tsx                   → layout with SettingsNav + <Outlet />
 *   src/routes/_app/settings/general.tsx                 → GeneralSettingsScreen
 *   src/features/settings/general/hooks/use-workspace-settings.ts
 *   src/features/settings/general/components/workspace-profile-card.tsx, notification-settings-card.tsx, delete-workspace-card.tsx
 */
import * as React from "react";
import { Link, useBlocker, useLocation, useNavigate } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { CircleAlert, Info, Lock } from "lucide-react";
import { Controller, useForm } from "react-hook-form";
import { toast } from "sonner";
import { z } from "zod";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { ApiError, apiClient } from "@/lib/api-client";
import { cn } from "@/lib/utils";

// -----------------------------------------------------------------------------
// Types and hooks — each card owns its own query (per-card S1/S4)
// -----------------------------------------------------------------------------

interface WorkspaceProfile {
  name: string;
  slug: string;
  defaultCurrency: "EUR" | "USD" | "GBP";
  canEdit: boolean;
  pendingPropagation?: boolean; // S6: changes still applying to members
}

interface NotificationSettings {
  weeklyDigest: boolean;
  failedPaymentAlerts: boolean;
  newMemberAlerts: boolean;
  canEdit: boolean;
}

const settingsKeys = {
  profile: ["settings", "workspace-profile"] as const,
  notifications: ["settings", "notifications"] as const,
};

function useWorkspaceProfile() {
  return useQuery({ queryKey: settingsKeys.profile, queryFn: () => apiClient.get<WorkspaceProfile>("/workspace/profile") });
}

function useNotificationSettings() {
  return useQuery({ queryKey: settingsKeys.notifications, queryFn: () => apiClient.get<NotificationSettings>("/workspace/notifications") });
}

// -----------------------------------------------------------------------------
// Settings navigation (max 2 levels; replaces page tabs). Mobile → Select.
// -----------------------------------------------------------------------------

const settingsGroups = [
  { audience: "Workspace", items: [{ to: "/settings/general", label: "General" }, { to: "/settings/members", label: "Members" }, { to: "/settings/billing", label: "Billing" }] },
  { audience: "Personal", items: [{ to: "/settings/profile", label: "Profile" }, { to: "/settings/preferences", label: "Preferences" }] },
  { audience: "Developers", items: [{ to: "/settings/api-keys", label: "API keys" }] },
] as const;

export function SettingsNav() {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  return (
    <>
      <div className="lg:hidden">
        <Select value={pathname} onValueChange={(to) => navigate({ to })}>
          <SelectTrigger aria-label="Settings section" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {settingsGroups.flatMap((group) =>
              group.items.map((item) => (
                <SelectItem key={item.to} value={item.to}>
                  {group.audience} · {item.label}
                </SelectItem>
              )),
            )}
          </SelectContent>
        </Select>
      </div>
      <nav aria-label="Settings" className="hidden lg:block">
        <div className="space-y-6">
          {settingsGroups.map((group) => (
            <div key={group.audience} className="space-y-1">
              <p className="px-2 text-xs font-medium text-muted-foreground">{group.audience}</p>
              <ul className="space-y-0.5">
                {group.items.map((item) => {
                  const isActive = pathname === item.to;
                  return (
                    <li key={item.to}>
                      <Link
                        to={item.to}
                        aria-current={isActive ? "page" : undefined}
                        className={cn(
                          "block rounded-md px-2 py-1.5 text-sm transition-colors hover:bg-muted",
                          isActive ? "bg-muted font-medium text-foreground" : "text-muted-foreground",
                        )}
                      >
                        {item.label}
                      </Link>
                    </li>
                  );
                })}
              </ul>
            </div>
          ))}
        </div>
      </nav>
    </>
  );
}

// -----------------------------------------------------------------------------
// Shared per-card pieces
// -----------------------------------------------------------------------------

function CardSkeleton({ rows }: { rows: number }) {
  return (
    <Card aria-hidden="true">
      <CardHeader>
        <Skeleton className="h-5 w-40" />
        <Skeleton className="h-4 w-72" />
      </CardHeader>
      <CardContent className="space-y-4">
        {Array.from({ length: rows }).map((_, index) => (
          <div key={index} className="space-y-2">
            <Skeleton className="h-4 w-28" />
            <Skeleton className="h-9 w-full" />
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

function CardLoadError({ title, onRetry }: { title: string; onRetry: () => void }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <Alert variant="destructive" role="status">
          <CircleAlert aria-hidden="true" />
          <AlertTitle>Couldn't load {title.toLowerCase()}</AlertTitle>
          <AlertDescription>
            <Button variant="outline" size="sm" className="mt-2" onClick={onRetry}>
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      </CardContent>
    </Card>
  );
}

function AdminOnlyHint() {
  // S5: read-only fields with a reason
  return (
    <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
      <Lock className="size-3.5" aria-hidden="true" />
      Only admins can change this
    </p>
  );
}

// -----------------------------------------------------------------------------
// Card 1 — per-card save model (AC-SET-1, AC-SET-3)
// -----------------------------------------------------------------------------

const profileSchema = z.object({
  name: z.string().trim().min(2, "Enter a workspace name of at least 2 characters."),
  slug: z
    .string()
    .trim()
    .regex(/^[a-z0-9-]{3,32}$/, "Use 3–32 lowercase letters, numbers, or hyphens."),
  defaultCurrency: z.enum(["EUR", "USD", "GBP"]),
});
type ProfileValues = z.infer<typeof profileSchema>;

function WorkspaceProfileCard() {
  const profile = useWorkspaceProfile();
  const queryClient = useQueryClient();
  const save = useMutation({
    mutationFn: (values: ProfileValues) => apiClient.patch<WorkspaceProfile>("/workspace/profile", values),
    onSuccess: (saved) => queryClient.setQueryData(settingsKeys.profile, saved),
  });
  const form = useForm<ProfileValues>({
    resolver: zodResolver(profileSchema),
    values: profile.data ? { name: profile.data.name, slug: profile.data.slug, defaultCurrency: profile.data.defaultCurrency } : undefined,
    mode: "onTouched",
  });
  const { isDirty } = form.formState;

  // AC-SET-3: leaving with a dirty card prompts.
  const blocker = useBlocker({ shouldBlockFn: () => form.formState.isDirty, enableBeforeUnload: () => form.formState.isDirty, withResolver: true });

  if (profile.isPending) return <CardSkeleton rows={3} />;
  if (profile.isError) return <CardLoadError title="Workspace profile" onRetry={() => profile.refetch()} />;

  const readOnly = !profile.data.canEdit;

  const onSubmit = form.handleSubmit(async (values) => {
    try {
      const saved = await save.mutateAsync(values);
      form.reset({ name: saved.name, slug: saved.slug, defaultCurrency: saved.defaultCurrency });
      toast.success("Workspace profile saved");
    } catch (error) {
      if (error instanceof ApiError && error.fieldErrors?.slug) {
        form.setError("slug", { type: "server", message: error.fieldErrors.slug }, { shouldFocus: true });
      } else {
        form.setError("root", { message: "Couldn't save changes. Your edits are kept; try again." });
      }
    }
  });

  return (
    <Card>
      <form noValidate onSubmit={onSubmit}>
        <CardHeader>
          <CardTitle>Workspace profile</CardTitle>
          <CardDescription>How your workspace appears to members and on documents.</CardDescription>
        </CardHeader>
        <CardContent className="py-6">
          {form.formState.errors.root ? (
            <Alert variant="destructive" role="alert" className="mb-4">
              <CircleAlert aria-hidden="true" />
              <AlertDescription>{form.formState.errors.root.message}</AlertDescription>
            </Alert>
          ) : null}
          <FieldGroup>
            <Controller
              control={form.control}
              name="name"
              render={({ field, fieldState }) => (
                <Field data-invalid={fieldState.invalid}>
                  <FieldLabel htmlFor="workspace-name">Workspace name</FieldLabel>
                  <Input {...field} id="workspace-name" readOnly={readOnly} aria-invalid={fieldState.invalid} />
                  {readOnly ? <AdminOnlyHint /> : null}
                  <FieldError errors={[fieldState.error]} />
                </Field>
              )}
            />
            <Controller
              control={form.control}
              name="slug"
              render={({ field, fieldState }) => (
                <Field data-invalid={fieldState.invalid}>
                  <FieldLabel htmlFor="workspace-slug">Workspace URL</FieldLabel>
                  <Input {...field} id="workspace-slug" readOnly={readOnly} className="font-mono" aria-invalid={fieldState.invalid} aria-describedby="workspace-slug-hint" />
                  <FieldDescription id="workspace-slug-hint">Changing it breaks existing bookmarks to this workspace.</FieldDescription>
                  <FieldError errors={[fieldState.error]} />
                </Field>
              )}
            />
            <Controller
              control={form.control}
              name="defaultCurrency"
              render={({ field }) => (
                <Field>
                  <FieldLabel htmlFor="default-currency">Default currency</FieldLabel>
                  <Select value={field.value} onValueChange={field.onChange} disabled={readOnly}>
                    <SelectTrigger id="default-currency" className="w-48">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="EUR">Euro (EUR)</SelectItem>
                      <SelectItem value="USD">US dollar (USD)</SelectItem>
                      <SelectItem value="GBP">Pound sterling (GBP)</SelectItem>
                    </SelectContent>
                  </Select>
                  <FieldDescription>Used for new invoices and reports.</FieldDescription>
                </Field>
              )}
            />
          </FieldGroup>
        </CardContent>
        {!readOnly ? (
          <CardFooter className="justify-end gap-2 border-t border-border pt-6">
            <Button type="button" variant="outline" disabled={!isDirty || save.isPending} onClick={() => form.reset()}>
              Cancel
            </Button>
            {/* AC-SET-1: Save disabled while the card is clean; enabled when dirty */}
            <Button type="submit" disabled={!isDirty || save.isPending} className="min-w-24">
              {save.isPending ? <Spinner /> : null}
              {save.isPending ? "Saving…" : "Save"}
            </Button>
          </CardFooter>
        ) : null}
      </form>

      <AlertDialog open={blocker.status === "blocked"}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Discard changes?</AlertDialogTitle>
            <AlertDialogDescription>Your edits to the workspace profile haven't been saved.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => blocker.reset?.()}>Keep editing</AlertDialogCancel>
            <AlertDialogAction onClick={() => blocker.proceed?.()}>Discard</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}

// -----------------------------------------------------------------------------
// Card 2 — instant save model (AC-SET-1): optimistic, inline "Saved", revert on failure
// -----------------------------------------------------------------------------

type NotificationKey = "weeklyDigest" | "failedPaymentAlerts" | "newMemberAlerts";

const notificationRows: { key: NotificationKey; label: string; hint: string }[] = [
  { key: "weeklyDigest", label: "Weekly digest", hint: "A Monday summary of activity for all admins." },
  { key: "failedPaymentAlerts", label: "Failed payment alerts", hint: "Email billing admins when a charge fails." },
  { key: "newMemberAlerts", label: "New member alerts", hint: "Email admins when someone joins the workspace." },
];

function NotificationSettingsCard() {
  const settings = useNotificationSettings();
  const queryClient = useQueryClient();
  const [savedKey, setSavedKey] = React.useState<NotificationKey>();
  const [failedKey, setFailedKey] = React.useState<NotificationKey>();

  const toggle = useMutation({
    mutationFn: ({ key, value }: { key: NotificationKey; value: boolean }) => apiClient.patch<NotificationSettings>("/workspace/notifications", { [key]: value }),
    onMutate: async ({ key, value }) => {
      setFailedKey(undefined);
      await queryClient.cancelQueries({ queryKey: settingsKeys.notifications });
      const previous = queryClient.getQueryData<NotificationSettings>(settingsKeys.notifications);
      queryClient.setQueryData<NotificationSettings>(settingsKeys.notifications, (old) => (old ? { ...old, [key]: value } : old));
      return { previous };
    },
    onError: (_error, { key }, context) => {
      queryClient.setQueryData(settingsKeys.notifications, context?.previous); // revert the control
      setFailedKey(key);
    },
    onSuccess: (_data, { key }) => {
      setSavedKey(key);
      window.setTimeout(() => setSavedKey((current) => (current === key ? undefined : current)), 2000);
    },
  });

  if (settings.isPending) return <CardSkeleton rows={3} />;
  if (settings.isError) return <CardLoadError title="Notifications" onRetry={() => settings.refetch()} />;

  return (
    <Card>
      <CardHeader>
        <CardTitle>Notifications</CardTitle>
        <CardDescription>Changes apply immediately.</CardDescription>
      </CardHeader>
      <CardContent>
        <ul className="divide-y divide-border">
          {notificationRows.map((row) => (
            <li key={row.key} className="flex items-start justify-between gap-6 py-4 first:pt-0 last:pb-0">
              <div className="space-y-1">
                <label htmlFor={`notify-${row.key}`} className="text-sm font-medium">
                  {row.label}
                </label>
                <p id={`notify-${row.key}-hint`} className="text-xs text-muted-foreground">
                  {row.hint}
                </p>
                {failedKey === row.key ? (
                  <p role="alert" className="flex items-center gap-1 text-xs text-destructive">
                    <CircleAlert className="size-3.5" aria-hidden="true" />
                    Couldn't save this setting. Try again.
                  </p>
                ) : null}
                {!settings.data.canEdit ? <AdminOnlyHint /> : null}
              </div>
              <div className="flex items-center gap-2">
                <span role="status" className="text-xs text-muted-foreground">
                  {savedKey === row.key ? "Saved" : ""}
                </span>
                <Switch
                  id={`notify-${row.key}`}
                  aria-describedby={`notify-${row.key}-hint`}
                  checked={settings.data[row.key]}
                  disabled={!settings.data.canEdit}
                  onCheckedChange={(value) => toggle.mutate({ key: row.key, value })}
                />
              </div>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}

// -----------------------------------------------------------------------------
// Card 3 — danger zone, always last (AC-SET-2): org-destructive-confirm + mol-confirm-input
// -----------------------------------------------------------------------------

function DeleteWorkspaceCard({ workspaceName, canDelete }: { workspaceName: string; canDelete: boolean }) {
  const [open, setOpen] = React.useState(false);
  const [confirmText, setConfirmText] = React.useState("");
  const remove = useMutation({ mutationFn: () => apiClient.delete<void>("/workspace") });

  if (!canDelete) return null; // never grantable for this user → hidden

  return (
    <Card className="border-destructive/40">
      <CardHeader>
        <CardTitle>Danger zone</CardTitle>
        <CardDescription>Irreversible actions for this workspace.</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="space-y-1">
          <p className="text-sm font-medium">Delete workspace</p>
          <p className="text-xs text-muted-foreground">Permanently delete all projects, invoices, and members' access.</p>
        </div>
        {/* Danger subtle entry point; the solid danger button lives only inside the confirmation */}
        <Button variant="outline" className="text-destructive hover:text-destructive" onClick={() => setOpen(true)}>
          Delete workspace
        </Button>
      </CardContent>

      <AlertDialog
        open={open}
        onOpenChange={(next) => {
          if (remove.isPending) return;
          setOpen(next);
          if (!next) setConfirmText("");
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete workspace '{workspaceName}'?</AlertDialogTitle>
            <AlertDialogDescription>
              All projects, invoices, files, and member access are deleted for everyone. This can't be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <Field>
            <FieldLabel htmlFor="confirm-workspace-name">
              Type <strong className="font-mono">{workspaceName}</strong> to confirm
            </FieldLabel>
            <Input id="confirm-workspace-name" autoComplete="off" spellCheck={false} value={confirmText} onChange={(event) => setConfirmText(event.target.value)} />
          </Field>
          {remove.isError ? (
            <Alert variant="destructive" role="alert">
              <CircleAlert aria-hidden="true" />
              <AlertDescription>Couldn't delete the workspace. Nothing was deleted; try again.</AlertDescription>
            </Alert>
          ) : null}
          <AlertDialogFooter>
            <AlertDialogCancel autoFocus disabled={remove.isPending}>
              Cancel
            </AlertDialogCancel>
            {/* Button (not AlertDialogAction) keeps the dialog open while pending and on failure */}
            <Button
              variant="destructive"
              disabled={confirmText !== workspaceName || remove.isPending}
              onClick={() => remove.mutate(undefined, { onSuccess: () => window.location.assign("/") })}
            >
              {remove.isPending ? <Spinner /> : null}
              Delete workspace
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}

// -----------------------------------------------------------------------------
// Screen entry — tpl-settings (General)
// -----------------------------------------------------------------------------

export function GeneralSettingsScreen() {
  const profile = useWorkspaceProfile();

  return (
    <div className="mx-auto grid w-full max-w-[1100px] gap-8 px-4 py-6 lg:grid-cols-[220px_minmax(0,720px)] lg:px-8">
      <SettingsNav />
      <div className="flex flex-col gap-6">
        <header className="space-y-1">
          <h1 className="text-xl font-semibold tracking-tight">General</h1>
          <p className="text-sm text-muted-foreground">Workspace identity, notifications, and deletion.</p>
        </header>

        {profile.data?.pendingPropagation ? (
          // S6: pending changes awaiting propagation
          <Alert role="status">
            <Info aria-hidden="true" />
            <AlertTitle>Changes are still applying</AlertTitle>
            <AlertDescription>Members may see the previous workspace URL for a few minutes.</AlertDescription>
          </Alert>
        ) : null}

        <WorkspaceProfileCard />
        <NotificationSettingsCard />
        {profile.data ? <DeleteWorkspaceCard workspaceName={profile.data.name} canDelete={profile.data.canEdit} /> : null}
      </div>
    </div>
  );
}
