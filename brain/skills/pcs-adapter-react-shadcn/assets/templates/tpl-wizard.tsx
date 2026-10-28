/**
 * Blueprint: `tpl-wizard` — Multi-Step Wizard (pcs-saas 01 §8)
 *
 * Domain: Set up a data sync connection. Step 2's fields depend on the source chosen in step 1,
 * and a connection test must pass before commit → a wizard, not a long form.
 *
 * Catalog nodes composed here:
 *   minimal shell (logo + "Exit" with dirty guard) · org-wizard-stepper (full + compact)
 *   org-sectioned-form (one step = one section group, <= 7 fields) · mol-form-field
 *   review step (mol-key-value-list per step with "Edit" links) · mol-button-group (Back / Continue)
 *   mol-progress-bar (commit) · mol-inline-alert · org-feedback · org-empty-state (completion)
 *
 * Acceptance checks satisfied (search for the tag):
 *   AC-WIZ-1 Step count <= 5 and visible progress shows current step and total.
 *   AC-WIZ-2 "Back" preserves every entered value (one form instance across steps).
 *   AC-WIZ-3 Review step lists all answers with edit links to their steps.
 *
 * Split into a project:
 *   src/routes/_wizard/connections/new.tsx               → Route (validateSearch: wizardSearchSchema), minimal layout route
 *   src/features/integrations/connections/schemas/connection-wizard.schema.ts
 *   src/features/integrations/connections/hooks/use-connection-wizard.ts (draft, test, commit)
 *   src/features/integrations/connections/components/connection-wizard-stepper.tsx, steps/*.tsx, review-step.tsx
 *   src/features/integrations/connections/new/index.tsx → ConnectionWizardScreen
 */
import * as React from "react";
import { getRouteApi, Link, useBlocker } from "@tanstack/react-router";
import { useMutation, useQuery } from "@tanstack/react-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { Check, CircleAlert, CircleCheck, LockKeyhole, X } from "lucide-react";
import { Controller, useForm, useWatch, type FieldPath } from "react-hook-form";
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
import { Card, CardContent, CardFooter, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Progress } from "@/components/ui/progress";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { ApiError, apiClient } from "@/lib/api-client";
import { cn } from "@/lib/utils";

// -----------------------------------------------------------------------------
// Schema — one form across all steps so Back never loses data (AC-WIZ-2)
// -----------------------------------------------------------------------------

export const connectionWizardSchema = z
  .object({
    sourceType: z.enum(["database", "storage-bucket", "rest-api"], { error: "Choose where the data comes from." }),
    name: z.string().trim().min(3, "Enter a name of at least 3 characters, e.g. Orders database."),
    host: z.string().trim().optional(),
    port: z.coerce.number().int().min(1).max(65535).optional(),
    bucketPath: z.string().trim().optional(),
    baseUrl: z.string().trim().optional(),
    credentialId: z.string().min(1, "Choose the credential used to connect."),
    frequency: z.enum(["hourly", "daily", "weekly"]),
    startTime: z.string().regex(/^\d{2}:\d{2}$/, "Enter a time as HH:MM, e.g. 02:00."),
  })
  .superRefine((values, ctx) => {
    // Step 2 fields depend on step 1's answer.
    if (values.sourceType === "database" && !values.host) ctx.addIssue({ code: "custom", path: ["host"], message: "Enter the database host, e.g. db.internal.example.com." });
    if (values.sourceType === "database" && !values.port) ctx.addIssue({ code: "custom", path: ["port"], message: "Enter the port number, e.g. 5432." });
    if (values.sourceType === "storage-bucket" && !values.bucketPath) ctx.addIssue({ code: "custom", path: ["bucketPath"], message: "Enter the bucket path, e.g. exports/orders/." });
    if (values.sourceType === "rest-api" && !values.baseUrl?.startsWith("https://")) ctx.addIssue({ code: "custom", path: ["baseUrl"], message: "Enter an address that starts with https://." });
  });

export type ConnectionWizardValues = z.output<typeof connectionWizardSchema>;
// z.coerce makes the input type differ from the output type (zod 4).
type ConnectionWizardInput = z.input<typeof connectionWizardSchema>;

const steps = [
  { id: "source", name: "Source", fields: ["sourceType", "name"] },
  { id: "connection", name: "Connection", fields: ["host", "port", "bucketPath", "baseUrl", "credentialId"] },
  { id: "schedule", name: "Schedule", fields: ["frequency", "startTime"] },
  { id: "review", name: "Review", fields: [] },
] as const satisfies readonly { id: string; name: string; fields: readonly FieldPath<ConnectionWizardValues>[] }[];

type StepId = (typeof steps)[number]["id"];

export const wizardSearchSchema = z.object({
  step: z.enum(["source", "connection", "schedule", "review"]).default("source").catch("source"),
  draftId: z.string().optional().catch(undefined),
});

const routeApi = getRouteApi("/_wizard/connections/new");

// -----------------------------------------------------------------------------
// Hooks — permission check, draft persistence per step (> 3 steps), commit
// -----------------------------------------------------------------------------

interface WizardContext {
  canCreate: boolean;
  credentials: { id: string; label: string }[];
  draft?: { values: Partial<ConnectionWizardInput>; completedSteps: StepId[] };
}

interface CommitResult {
  connectionId: string;
  tablesSynced: number;
  failures: { table: string; reason: string }[]; // non-empty → partial commit (S6)
}

function useWizardContext(draftId: string | undefined) {
  return useQuery({
    queryKey: ["connections", "wizard-context", draftId ?? "new"],
    queryFn: () => apiClient.get<WizardContext>("/connections/wizard-context", { params: { draftId } }),
  });
}

function useSaveDraft() {
  return useMutation({
    mutationFn: (payload: { draftId?: string; values: Partial<ConnectionWizardInput>; completedSteps: StepId[] }) =>
      apiClient.post<{ draftId: string }>("/connections/drafts", payload),
  });
}

function useCommitConnection() {
  return useMutation({
    mutationFn: (values: ConnectionWizardValues) => apiClient.post<CommitResult>("/connections", values),
  });
}

// -----------------------------------------------------------------------------
// org-wizard-stepper — ordered list, aria-current="step", state in text (AC-WIZ-1)
// -----------------------------------------------------------------------------

function WizardStepper({
  current,
  completed,
  errored,
  onGoTo,
}: {
  current: StepId;
  completed: StepId[];
  errored: StepId[];
  onGoTo: (step: StepId) => void;
}) {
  const currentIndex = steps.findIndex((step) => step.id === current);
  return (
    <nav aria-label="Progress">
      {/* Compact variant below 640px */}
      <div className="space-y-2 sm:hidden">
        <p className="text-sm font-medium">
          Step <span className="tabular-nums">{currentIndex + 1}</span> of <span className="tabular-nums">{steps.length}</span> — {steps[currentIndex].name}
        </p>
        <Progress value={((currentIndex + 1) / steps.length) * 100} aria-label={`Step ${currentIndex + 1} of ${steps.length}`} />
      </div>
      <ol className="hidden items-center gap-2 sm:flex">
        {steps.map((step, index) => {
          const isCurrent = step.id === current;
          const isCompleted = completed.includes(step.id) && !isCurrent;
          const hasError = errored.includes(step.id);
          const stateText = hasError ? "Has errors" : isCurrent ? "Current" : isCompleted ? "Completed" : "Not started";
          const marker = (
            <>
              <span
                aria-hidden="true"
                className={cn(
                  "flex size-6 shrink-0 items-center justify-center rounded-full border text-xs tabular-nums",
                  isCurrent && "border-primary bg-primary text-primary-foreground",
                  isCompleted && "border-primary text-primary",
                  hasError && "border-destructive text-destructive",
                )}
              >
                {isCompleted && !hasError ? <Check className="size-3.5" /> : index + 1}
              </span>
              <span className={cn("text-sm", isCurrent ? "font-medium" : "text-muted-foreground", hasError && "text-destructive")}>{step.name}</span>
              <span className="sr-only">({stateText})</span>
            </>
          );
          return (
            <React.Fragment key={step.id}>
              <li aria-current={isCurrent ? "step" : undefined} className="flex items-center gap-2">
                {/* Completed steps are clickable to go back; upcoming steps are not */}
                {isCompleted ? (
                  <Button variant="ghost" size="sm" className="h-auto gap-2 px-1 py-1" onClick={() => onGoTo(step.id)}>
                    {marker}
                  </Button>
                ) : (
                  <span className="flex items-center gap-2 px-1 py-1">{marker}</span>
                )}
              </li>
              {index < steps.length - 1 ? <li aria-hidden="true" className="h-px flex-1 bg-border" /> : null}
            </React.Fragment>
          );
        })}
      </ol>
    </nav>
  );
}

// -----------------------------------------------------------------------------
// Screen entry — tpl-wizard
// -----------------------------------------------------------------------------

export function ConnectionWizardScreen() {
  const search = routeApi.useSearch();
  const navigate = routeApi.useNavigate();
  const context = useWizardContext(search.draftId);
  const saveDraft = useSaveDraft();
  const commit = useCommitConnection();
  const [completed, setCompleted] = React.useState<StepId[]>([]);
  const [result, setResult] = React.useState<CommitResult>();
  const [commitError, setCommitError] = React.useState<string>();
  const stepHeadingRef = React.useRef<HTMLHeadingElement>(null);

  const form = useForm<ConnectionWizardInput, unknown, ConnectionWizardValues>({
    resolver: zodResolver(connectionWizardSchema),
    defaultValues: { frequency: "daily", startTime: "02:00", name: "", credentialId: "" },
    mode: "onTouched",
  });
  const sourceType = useWatch({ control: form.control, name: "sourceType" });

  // Resume a draft at the last incomplete step.
  React.useEffect(() => {
    const draft = context.data?.draft;
    if (!draft) return;
    form.reset({ ...form.getValues(), ...draft.values });
    setCompleted(draft.completedSteps);
  }, [context.data?.draft, form]);

  // A step in the URL is valid only when every earlier step is completed (no jumping forward).
  const requestedIndex = steps.findIndex((step) => step.id === search.step);
  const firstIncomplete = steps.findIndex((step) => !completed.includes(step.id));
  const stepIndex = Math.min(requestedIndex, firstIncomplete === -1 ? steps.length - 1 : firstIncomplete);
  const step = steps[stepIndex];

  const goTo = (id: StepId) => {
    navigate({ search: (prev) => ({ ...prev, step: id }) });
    requestAnimationFrame(() => stepHeadingRef.current?.focus()); // move focus to the new step heading
  };

  const blocker = useBlocker({
    shouldBlockFn: () => form.formState.isDirty && !result,
    enableBeforeUnload: () => form.formState.isDirty && !result,
    withResolver: true,
  });

  // Validate the current step only, in order (never all steps at the end).
  const onContinue = async () => {
    const valid = await form.trigger(step.fields as unknown as FieldPath<ConnectionWizardValues>[], { shouldFocus: true });
    if (!valid) return;
    const nextCompleted = Array.from(new Set([...completed, step.id]));
    setCompleted(nextCompleted);
    const saved = await saveDraft.mutateAsync({ draftId: search.draftId, values: form.getValues(), completedSteps: nextCompleted }).catch(() => undefined);
    navigate({ search: (prev) => ({ ...prev, draftId: saved?.draftId ?? prev.draftId, step: steps[stepIndex + 1].id }) });
    requestAnimationFrame(() => stepHeadingRef.current?.focus());
  };

  const onCommit = form.handleSubmit(async (values) => {
    setCommitError(undefined);
    try {
      setResult(await commit.mutateAsync(values));
    } catch (error) {
      // Commit failure returns to the review step with alert and preserved inputs.
      setCommitError(error instanceof ApiError ? error.message : "The connection couldn't be created. Your answers are kept.");
    }
  });

  // ---- S1 ----
  if (context.isPending) {
    return (
      <WizardFrame>
        <div aria-hidden="true" className="space-y-6">
          <Skeleton className="h-6 w-full" />
          <Skeleton className="h-72 w-full rounded-xl" />
        </div>
      </WizardFrame>
    );
  }

  // ---- S5: permission check before step 1 ----
  if (context.data && !context.data.canCreate) {
    return (
      <WizardFrame>
        <Empty>
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <LockKeyhole aria-hidden="true" />
            </EmptyMedia>
            <EmptyTitle>You can't create connections</EmptyTitle>
            <EmptyDescription>Workspace admins can create connections or give you the Integrations role.</EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button asChild>
              <Link to="/connections">Go to connections</Link>
            </Button>
          </EmptyContent>
        </Empty>
      </WizardFrame>
    );
  }

  if (context.isError) {
    return (
      <WizardFrame>
        <Alert variant="destructive" role="status">
          <CircleAlert aria-hidden="true" />
          <AlertTitle>Couldn't start the setup</AlertTitle>
          <AlertDescription>
            <Button variant="outline" size="sm" className="mt-2" onClick={() => context.refetch()}>
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      </WizardFrame>
    );
  }

  // ---- Commit pending: say what is happening, never a blank screen ----
  if (commit.isPending) {
    return (
      <WizardFrame>
        <Card>
          <CardHeader>
            <CardTitle>Creating your connection</CardTitle>
            <CardDescription>Testing access and running the first sync. This usually takes under a minute.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2" role="status">
            <Progress value={undefined} aria-label="Creating connection" className="[&>[data-slot=progress-indicator]]:animate-pulse" />
            <p className="text-sm text-muted-foreground">You can leave this page; we'll notify you when it's done.</p>
          </CardContent>
        </Card>
      </WizardFrame>
    );
  }

  // ---- S6 partial commit / S7 completion ----
  if (result) {
    const isPartial = result.failures.length > 0;
    return (
      <WizardFrame>
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              {isPartial ? <CircleAlert className="size-5 text-warning" aria-hidden="true" /> : <CircleCheck className="size-5 text-success" aria-hidden="true" />}
              {isPartial ? "Connection created with some failures" : "Connection created"}
            </CardTitle>
            <CardDescription>
              <span className="tabular-nums">{result.tablesSynced}</span> tables synced
              {isPartial ? (
                <>
                  , <span className="tabular-nums">{result.failures.length}</span> failed
                </>
              ) : null}
              .
            </CardDescription>
          </CardHeader>
          {isPartial ? (
            <CardContent>
              <ul className="divide-y divide-border rounded-lg border border-border text-sm">
                {result.failures.map((failure) => (
                  <li key={failure.table} className="flex items-center justify-between gap-4 p-3">
                    <span className="font-mono">{failure.table}</span>
                    <span className="text-muted-foreground">{failure.reason}</span>
                  </li>
                ))}
              </ul>
            </CardContent>
          ) : null}
          <CardFooter className="justify-end gap-2">
            {isPartial ? <Button variant="outline">Retry failed tables</Button> : null}
            <Button asChild>
              <Link to="/connections/$connectionId" params={{ connectionId: result.connectionId }}>
                Open connection
              </Link>
            </Button>
          </CardFooter>
        </Card>
      </WizardFrame>
    );
  }

  const values = form.getValues();
  const credentialLabel = context.data?.credentials.find((c) => c.id === values.credentialId)?.label;
  const erroredSteps = steps.filter((s) => s.fields.some((name) => form.formState.errors[name as keyof ConnectionWizardValues])).map((s) => s.id);

  return (
    <WizardFrame onExit={() => navigate({ to: "/connections" })}>
      <WizardStepper current={step.id} completed={completed} errored={erroredSteps} onGoTo={goTo} />

      <form noValidate onSubmit={(event) => event.preventDefault()}>
        <Card>
          <CardHeader>
            {/* CardTitle renders a div; a focusable h2 with the same slot styling keeps heading semantics. */}
            <h2 ref={stepHeadingRef} tabIndex={-1} data-slot="card-title" className="leading-none font-semibold outline-none">
              {step.id === "source" ? "Choose a source" : step.id === "connection" ? "Connect to the source" : step.id === "schedule" ? "Set a schedule" : "Review and create"}
            </h2>
          </CardHeader>
          <CardContent>
            {step.id === "source" ? (
              <FieldGroup>
                <Controller
                  control={form.control}
                  name="sourceType"
                  render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                      <FieldLabel id="source-type-label">Source type</FieldLabel>
                      <RadioGroup value={field.value} onValueChange={field.onChange} aria-labelledby="source-type-label" aria-invalid={fieldState.invalid}>
                        {[
                          { value: "database", label: "Database", hint: "Sync tables from a SQL database." },
                          { value: "storage-bucket", label: "Storage bucket", hint: "Import files dropped into a folder." },
                          { value: "rest-api", label: "REST API", hint: "Pull records from an HTTPS endpoint." },
                        ].map((option) => (
                          <label key={option.value} className="flex cursor-pointer items-start gap-3 rounded-lg border border-border p-3 has-[[data-state=checked]]:border-primary">
                            <RadioGroupItem value={option.value} className="mt-0.5" />
                            <span>
                              <span className="block text-sm font-medium">{option.label}</span>
                              <span className="block text-xs text-muted-foreground">{option.hint}</span>
                            </span>
                          </label>
                        ))}
                      </RadioGroup>
                      <FieldError errors={[fieldState.error]} />
                    </Field>
                  )}
                />
                <Controller
                  control={form.control}
                  name="name"
                  render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                      <FieldLabel htmlFor="connection-name">Connection name</FieldLabel>
                      <Input {...field} id="connection-name" placeholder="e.g. Orders database" aria-invalid={fieldState.invalid} />
                      <FieldDescription>Shown in sync logs and alerts.</FieldDescription>
                      <FieldError errors={[fieldState.error]} />
                    </Field>
                  )}
                />
              </FieldGroup>
            ) : null}

            {step.id === "connection" ? (
              <FieldGroup>
                {sourceType === "database" ? (
                  <div className="grid gap-4 sm:grid-cols-[1fr_auto]">
                    <Controller
                      control={form.control}
                      name="host"
                      render={({ field, fieldState }) => (
                        <Field data-invalid={fieldState.invalid}>
                          <FieldLabel htmlFor="db-host">Host</FieldLabel>
                          <Input {...field} value={field.value ?? ""} id="db-host" autoFocus aria-invalid={fieldState.invalid} />
                          <FieldError errors={[fieldState.error]} />
                        </Field>
                      )}
                    />
                    <Controller
                      control={form.control}
                      name="port"
                      render={({ field, fieldState }) => (
                        <Field data-invalid={fieldState.invalid}>
                          <FieldLabel htmlFor="db-port">Port</FieldLabel>
                          <Input {...field} value={String(field.value ?? "")} id="db-port" inputMode="numeric" className="w-[10ch] tabular-nums" aria-invalid={fieldState.invalid} />
                          <FieldError errors={[fieldState.error]} />
                        </Field>
                      )}
                    />
                  </div>
                ) : sourceType === "storage-bucket" ? (
                  <Controller
                    control={form.control}
                    name="bucketPath"
                    render={({ field, fieldState }) => (
                      <Field data-invalid={fieldState.invalid}>
                        <FieldLabel htmlFor="bucket-path">Bucket path</FieldLabel>
                        <Input {...field} value={field.value ?? ""} id="bucket-path" autoFocus className="font-mono" aria-invalid={fieldState.invalid} />
                        <FieldError errors={[fieldState.error]} />
                      </Field>
                    )}
                  />
                ) : (
                  <Controller
                    control={form.control}
                    name="baseUrl"
                    render={({ field, fieldState }) => (
                      <Field data-invalid={fieldState.invalid}>
                        <FieldLabel htmlFor="base-url">Base URL</FieldLabel>
                        <Input {...field} value={field.value ?? ""} id="base-url" type="url" autoFocus placeholder="e.g. https://api.example.com/v2" aria-invalid={fieldState.invalid} />
                        <FieldError errors={[fieldState.error]} />
                      </Field>
                    )}
                  />
                )}
                <Controller
                  control={form.control}
                  name="credentialId"
                  render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                      <FieldLabel htmlFor="credential">Credential</FieldLabel>
                      <Select value={field.value} onValueChange={field.onChange}>
                        <SelectTrigger id="credential" className="w-full" aria-invalid={fieldState.invalid} onBlur={field.onBlur}>
                          <SelectValue placeholder="Choose a credential" />
                        </SelectTrigger>
                        <SelectContent>
                          {context.data?.credentials.map((credential) => (
                            <SelectItem key={credential.id} value={credential.id}>
                              {credential.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <FieldDescription>Stored encrypted; never shown again after saving.</FieldDescription>
                      <FieldError errors={[fieldState.error]} />
                    </Field>
                  )}
                />
              </FieldGroup>
            ) : null}

            {step.id === "schedule" ? (
              <FieldGroup>
                <Controller
                  control={form.control}
                  name="frequency"
                  render={({ field }) => (
                    <Field>
                      <FieldLabel htmlFor="frequency">Frequency</FieldLabel>
                      <Select value={field.value} onValueChange={field.onChange}>
                        <SelectTrigger id="frequency" className="w-48">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="hourly">Every hour</SelectItem>
                          <SelectItem value="daily">Every day</SelectItem>
                          <SelectItem value="weekly">Every week</SelectItem>
                        </SelectContent>
                      </Select>
                    </Field>
                  )}
                />
                <Controller
                  control={form.control}
                  name="startTime"
                  render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                      <FieldLabel htmlFor="start-time">Start time</FieldLabel>
                      <Input {...field} id="start-time" type="time" className="w-[14ch] tabular-nums" aria-invalid={fieldState.invalid} />
                      <FieldDescription>In your workspace timezone.</FieldDescription>
                      <FieldError errors={[fieldState.error]} />
                    </Field>
                  )}
                />
              </FieldGroup>
            ) : null}

            {step.id === "review" ? (
              // AC-WIZ-3: every answer listed with an edit link to its step
              <div className="space-y-6">
                {commitError ? (
                  <Alert variant="destructive" role="alert">
                    <CircleAlert aria-hidden="true" />
                    <AlertTitle>Connection not created</AlertTitle>
                    <AlertDescription>{commitError}</AlertDescription>
                  </Alert>
                ) : null}
                {[
                  { step: "source" as const, rows: [["Source type", values.sourceType], ["Connection name", values.name]] },
                  {
                    step: "connection" as const,
                    rows: [
                      ...(values.sourceType === "database" ? [["Host", `${values.host}:${values.port}`]] : []),
                      ...(values.sourceType === "storage-bucket" ? [["Bucket path", values.bucketPath]] : []),
                      ...(values.sourceType === "rest-api" ? [["Base URL", values.baseUrl]] : []),
                      ["Credential", credentialLabel],
                    ],
                  },
                  { step: "schedule" as const, rows: [["Frequency", values.frequency], ["Start time", values.startTime]] },
                ].map((group) => (
                  <section key={group.step} aria-labelledby={`review-${group.step}`} className="space-y-2">
                    <div className="flex items-center justify-between">
                      <h3 id={`review-${group.step}`} className="text-sm font-semibold">
                        {steps.find((s) => s.id === group.step)?.name}
                      </h3>
                      <Button variant="link" size="sm" onClick={() => goTo(group.step)} aria-label={`Edit ${steps.find((s) => s.id === group.step)?.name}`}>
                        Edit
                      </Button>
                    </div>
                    <dl className="grid grid-cols-[minmax(0,35%)_1fr] gap-x-4 gap-y-2 text-sm">
                      {group.rows.map(([label, value]) => (
                        <React.Fragment key={label}>
                          <dt className="text-xs text-muted-foreground">{label}</dt>
                          <dd className="tabular-nums">{value || <span aria-label="Not set">—</span>}</dd>
                        </React.Fragment>
                      ))}
                    </dl>
                  </section>
                ))}
              </div>
            ) : null}
          </CardContent>

          {/* Footer: Back (left), Continue / commit verb (primary, right) */}
          <CardFooter className="flex justify-between gap-2 border-t border-border pt-6">
            {stepIndex > 0 ? (
              <Button variant="outline" onClick={() => goTo(steps[stepIndex - 1].id)}>
                Back
              </Button>
            ) : (
              <span />
            )}
            {step.id === "review" ? (
              <Button onClick={onCommit}>Create connection</Button>
            ) : (
              // Never disable Continue to signal invalid input; validate and show errors instead.
              <Button onClick={onContinue} disabled={saveDraft.isPending}>
                {saveDraft.isPending ? <Spinner /> : null}
                Continue
              </Button>
            )}
          </CardFooter>
        </Card>
      </form>

      <AlertDialog open={blocker.status === "blocked"}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Exit setup?</AlertDialogTitle>
            <AlertDialogDescription>Your progress is saved as a draft up to the last completed step.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => blocker.reset?.()}>Keep editing</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                blocker.proceed?.();
                toast("Draft saved", { description: "Resume from Connections → Drafts." });
              }}
            >
              Exit setup
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </WizardFrame>
  );
}

/** Minimal shell variant: logo + "Exit" only; centered form column 640–720px. */
function WizardFrame({ children, onExit }: { children: React.ReactNode; onExit?: () => void }) {
  return (
    <div className="min-h-dvh bg-background">
      <header className="flex h-14 items-center justify-between border-b border-border px-4 lg:px-8">
        <span className="text-sm font-semibold">Product</span>
        <Button variant="ghost" size="sm" onClick={onExit} asChild={!onExit}>
          {onExit ? (
            <>
              <X className="size-4" aria-hidden="true" />
              Exit
            </>
          ) : (
            <Link to="/connections">
              <X className="size-4" aria-hidden="true" />
              Exit
            </Link>
          )}
        </Button>
      </header>
      <main id="main-content" className="mx-auto flex w-full max-w-[720px] flex-col gap-6 px-4 py-8">
        {children}
      </main>
    </div>
  );
}
