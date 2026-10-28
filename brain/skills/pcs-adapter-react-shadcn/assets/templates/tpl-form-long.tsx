/**
 * Blueprint: `tpl-form-long` — Long Sectioned Form Page (pcs-saas 01 §7)
 *        and `tpl-form-short` — Short Form in a Dialog or Sheet (pcs-saas 01 §6)
 *
 * Domain: Vendors (long create/edit page, 5 sections) and vendor Contacts (short create dialog,
 * edit sheet). Generic procurement objects.
 *
 * Catalog nodes composed here:
 *   mol-breadcrumbs · org-page-header (no primary: sticky footer owns it) · section index (wide, >= 5 sections)
 *   org-sectioned-form · mol-form-field · mol-inline-alert (error summary / submit error)
 *   mol-button-group (sticky footer, dialog footer) · org-banner (S5 read-only, S6 autosave)
 *   org-feedback (S7) · dirty guard (route leave, dialog close) · mol-skeleton
 *
 * Acceptance checks satisfied (search for the tag):
 *   AC-LONG-1  Every section holds 3–7 fields.
 *   AC-LONG-2  Submitting with errors moves focus to an error summary whose items link to fields.
 *   AC-LONG-3  Navigating away while dirty prompts; while clean does not.
 *   AC-SHORT-1 First field autofocused on open; focus returns to the trigger on close (Radix default).
 *   AC-SHORT-2 Escape closes a clean form and triggers the dirty guard on a modified form.
 *   AC-SHORT-3 Server error keeps all values and focuses the alert or the first invalid field.
 *
 * Split into a project:
 *   src/routes/_app/vendors/new.tsx, src/routes/_app/vendors/$vendorId/edit.tsx → VendorFormScreen
 *   src/features/procurement/vendors/schemas/vendor-form.schema.ts, contact-form.schema.ts
 *   src/features/procurement/vendors/hooks/use-vendor-form.ts (useVendor, useSaveVendor, useCreateContact)
 *   src/features/procurement/vendors/components/vendor-form.tsx, create-contact-dialog.tsx, edit-contact-sheet.tsx
 */
import * as React from "react";
import { Link, useBlocker, useNavigate } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { CircleAlert, Lock, TriangleAlert } from "lucide-react";
import { Controller, useForm, type FieldErrors, type UseFormReturn } from "react-hook-form";
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
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { ApiError, apiClient } from "@/lib/api-client";
import { cn } from "@/lib/utils";

// =============================================================================
// Part A — tpl-form-long
// =============================================================================

// Schema messages follow the error formula: what is wrong + how to fix (pcs-saas 05 §9.4).
export const vendorFormSchema = z.object({
  // Section 1 — Company (4 fields)
  legalName: z.string().trim().min(2, "Enter the vendor's legal name."),
  displayName: z.string().trim().optional(),
  website: z.union([z.literal(""), z.url("Enter a full web address, e.g. https://example.com.")]),
  category: z.enum(["software", "services", "hardware", "logistics"], { error: "Choose a category." }),
  // Section 2 — Primary contact (3 fields)
  contactName: z.string().trim().min(2, "Enter the contact's full name."),
  contactEmail: z.email("Enter an email address like name@example.com."),
  contactPhone: z.string().trim().optional(),
  // Section 3 — Address (4 fields)
  street: z.string().trim().min(3, "Enter the street address."),
  city: z.string().trim().min(2, "Enter the city."),
  postalCode: z.string().trim().min(3, "Enter the postal code."),
  country: z.string().length(2, "Choose a country."),
  // Section 4 — Payment (3 fields)
  currency: z.enum(["EUR", "USD", "GBP"], { error: "Choose a payment currency." }),
  paymentTermsDays: z.coerce.number().int().min(0, "Payment terms can't be negative.").max(120, "Payment terms can be at most 120 days."),
  iban: z.string().trim().regex(/^[A-Z]{2}\d{2}[A-Z0-9]{10,30}$/, "Enter an IBAN without spaces, e.g. DE89370400440532013000."),
  // Section 5 — Tax and notes (3 fields)
  taxId: z.string().trim().min(4, "Enter the tax ID shown on the vendor's invoices."),
  taxExempt: z.enum(["no", "yes"]),
  notes: z.string().trim().max(500, "Keep notes under 500 characters.").optional(),
});

export type VendorFormValues = z.output<typeof vendorFormSchema>;
// z.coerce makes the input type differ from the output type (zod 4): the form
// holds input values and the resolver hands submit handlers parsed output.
type VendorFormInput = z.input<typeof vendorFormSchema>;
type VendorFieldName = keyof VendorFormValues;

const vendorDefaults: VendorFormValues = {
  legalName: "",
  displayName: "",
  website: "",
  category: "services",
  contactName: "",
  contactEmail: "",
  contactPhone: "",
  street: "",
  city: "",
  postalCode: "",
  country: "",
  currency: "EUR",
  paymentTermsDays: 30, // default prefilled where the system can infer it
  iban: "",
  taxId: "",
  taxExempt: "no",
  notes: "",
};

// Section map drives the section index and the error summary order (AC-LONG-1: 3–7 fields each).
const vendorSections: { id: string; title: string; description: string; fields: VendorFieldName[] }[] = [
  { id: "company", title: "Company", description: "How this vendor appears on purchase orders.", fields: ["legalName", "displayName", "website", "category"] },
  { id: "contact", title: "Primary contact", description: "Who receives purchase orders and remittance notices.", fields: ["contactName", "contactEmail", "contactPhone"] },
  { id: "address", title: "Address", description: "Registered address printed on documents.", fields: ["street", "city", "postalCode", "country"] },
  { id: "payment", title: "Payment", description: "How and when this vendor is paid.", fields: ["currency", "paymentTermsDays", "iban"] },
  { id: "tax", title: "Tax and notes", description: "Tax details used for compliance reports.", fields: ["taxId", "taxExempt", "notes"] },
];

const fieldLabels: Record<VendorFieldName, string> = {
  legalName: "Legal name",
  displayName: "Display name",
  website: "Website",
  category: "Category",
  contactName: "Contact name",
  contactEmail: "Contact email",
  contactPhone: "Contact phone",
  street: "Street address",
  city: "City",
  postalCode: "Postal code",
  country: "Country",
  currency: "Payment currency",
  paymentTermsDays: "Payment terms (days)",
  iban: "IBAN",
  taxId: "Tax ID",
  taxExempt: "Tax exempt",
  notes: "Internal notes",
};

interface VendorRecord extends VendorFormValues {
  id: string;
  canEdit: boolean;
}

export const vendorKeys = {
  all: ["vendors"] as const,
  detail: (id: string) => [...vendorKeys.all, "detail", id] as const,
};

function useVendor(vendorId: string | undefined) {
  return useQuery({
    queryKey: vendorKeys.detail(vendorId ?? "new"),
    queryFn: () => apiClient.get<VendorRecord>(`/vendors/${vendorId}`),
    enabled: Boolean(vendorId),
  });
}

function useSaveVendor(vendorId: string | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (values: VendorFormValues) =>
      vendorId ? apiClient.patch<VendorRecord>(`/vendors/${vendorId}`, values) : apiClient.post<VendorRecord>("/vendors", values),
    onSuccess: (vendor) => {
      queryClient.setQueryData(vendorKeys.detail(vendor.id), vendor);
      queryClient.invalidateQueries({ queryKey: [...vendorKeys.all, "list"] });
    },
  });
}

// -----------------------------------------------------------------------------
// Error summary (mol-inline-alert, role="alert") — AC-LONG-2
// -----------------------------------------------------------------------------

function ErrorSummary({
  errors,
  formError,
  summaryRef,
}: {
  errors: FieldErrors<VendorFormInput>;
  formError?: string;
  summaryRef: React.RefObject<HTMLDivElement | null>;
}) {
  const ordered = vendorSections.flatMap((section) => section.fields).filter((name) => errors[name]);
  if (ordered.length === 0 && !formError) return null;

  return (
    <Alert ref={summaryRef} variant="destructive" role="alert" tabIndex={-1} className="outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50">
      <CircleAlert aria-hidden="true" />
      <AlertTitle>{ordered.length > 0 ? `Fix ${ordered.length} ${ordered.length === 1 ? "field" : "fields"} to continue` : "Couldn't save the vendor"}</AlertTitle>
      <AlertDescription>
        {formError ? <p>{formError}</p> : null}
        {ordered.length > 0 ? (
          <ul className="mt-1 list-disc space-y-1 pl-4">
            {ordered.map((name) => (
              <li key={name}>
                <a href={`#vendor-${name}`} className="underline underline-offset-4">
                  {fieldLabels[name]}: {errors[name]?.message}
                </a>
              </li>
            ))}
          </ul>
        ) : null}
      </AlertDescription>
    </Alert>
  );
}

// -----------------------------------------------------------------------------
// Field renderer — mol-form-field (label → hint → control → error)
// Most fields are required, so optional ones are marked "(optional)" (mark the minority).
// -----------------------------------------------------------------------------

const optionalFields = new Set<VendorFieldName>(["displayName", "website", "contactPhone", "notes"]);
const fieldHints: Partial<Record<VendorFieldName, string>> = {
  displayName: "Shown in lists instead of the legal name.",
  iban: "Used for every payment run to this vendor.",
  paymentTermsDays: "Days between invoice date and due date.",
  notes: "Visible to your team only.",
};

function VendorField({
  form,
  name,
  readOnly,
  className,
}: {
  form: UseFormReturn<VendorFormInput, unknown, VendorFormValues>;
  name: VendorFieldName;
  readOnly: boolean;
  className?: string;
}) {
  const id = `vendor-${name}`;
  const hint = fieldHints[name];

  return (
    <Controller
      control={form.control}
      name={name}
      render={({ field, fieldState }) => {
        const describedBy = [hint ? `${id}-hint` : null, fieldState.error ? `${id}-error` : null].filter(Boolean).join(" ") || undefined;
        const controlProps = {
          id,
          "aria-invalid": fieldState.invalid,
          "aria-describedby": describedBy,
          "aria-required": !optionalFields.has(name),
        };
        return (
          <Field data-invalid={fieldState.invalid} className={className}>
            <FieldLabel htmlFor={id}>
              {fieldLabels[name]}
              {optionalFields.has(name) ? <span className="font-normal text-muted-foreground"> (optional)</span> : null}
            </FieldLabel>
            {name === "category" || name === "currency" || name === "taxExempt" ? (
              <Select value={String(field.value)} onValueChange={field.onChange} disabled={readOnly}>
                <SelectTrigger {...controlProps} className="w-full sm:w-64" onBlur={field.onBlur}>
                  <SelectValue placeholder="Choose…" />
                </SelectTrigger>
                <SelectContent>
                  {(name === "category"
                    ? ["software", "services", "hardware", "logistics"]
                    : name === "currency"
                      ? ["EUR", "USD", "GBP"]
                      : ["no", "yes"]
                  ).map((option) => (
                    <SelectItem key={option} value={option} className="capitalize">
                      {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : name === "notes" ? (
              <Textarea {...field} {...controlProps} value={String(field.value ?? "")} readOnly={readOnly} rows={4} />
            ) : (
              <Input
                {...field}
                {...controlProps}
                value={String(field.value ?? "")}
                readOnly={readOnly}
                type={name === "contactEmail" ? "email" : name === "contactPhone" ? "tel" : name === "website" ? "url" : name === "paymentTermsDays" ? "number" : "text"}
                inputMode={name === "paymentTermsDays" ? "numeric" : undefined}
                autoComplete={name === "contactEmail" ? "email" : name === "postalCode" ? "postal-code" : name === "street" ? "street-address" : "off"}
                placeholder={name === "website" ? "e.g. https://example.com" : undefined}
                // Control width matches expected content length (mol-form-field).
                className={cn(
                  (name === "postalCode" || name === "paymentTermsDays") && "w-[10ch] tabular-nums",
                  (name === "iban" || name === "taxId") && "font-mono tabular-nums",
                )}
              />
            )}
            {hint ? <FieldDescription id={`${id}-hint`}>{hint}</FieldDescription> : null}
            <FieldError id={`${id}-error`} errors={[fieldState.error]} />
          </Field>
        );
      }}
    />
  );
}

// -----------------------------------------------------------------------------
// Screen entry — tpl-form-long (create when vendorId is undefined, edit otherwise)
// -----------------------------------------------------------------------------

export function VendorFormScreen({ vendorId }: { vendorId?: string }) {
  const navigate = useNavigate();
  const vendor = useVendor(vendorId);
  const save = useSaveVendor(vendorId);
  const summaryRef = React.useRef<HTMLDivElement>(null);
  const [formError, setFormError] = React.useState<string>();
  const [savedAt, setSavedAt] = React.useState<number>();

  const form = useForm<VendorFormInput, unknown, VendorFormValues>({
    resolver: zodResolver(vendorFormSchema),
    defaultValues: vendorDefaults,
    values: vendor.data, // edit mode: reset to server values once loaded
    mode: "onTouched", // blur first, then revalidate on change (pcs-saas 03 §2)
    shouldFocusError: false, // focus goes to the summary instead (AC-LONG-2)
  });

  const isEdit = Boolean(vendorId);
  const isReadOnly = isEdit && vendor.data?.canEdit === false; // S5: can view, cannot edit
  const { isDirty, isSubmitting, errors } = form.formState;

  // AC-LONG-3: route change and browser unload prompt only while dirty.
  const blocker = useBlocker({
    shouldBlockFn: () => form.formState.isDirty && !save.isSuccess,
    enableBeforeUnload: () => form.formState.isDirty,
    withResolver: true,
  });

  const onSubmit = form.handleSubmit(
    async (values) => {
      setFormError(undefined);
      try {
        const saved = await save.mutateAsync(values);
        form.reset(saved); // clean → leaving no longer prompts
        if (isEdit) {
          setSavedAt(Date.now()); // S7 edit: stay and show "Saved" near the footer
        } else {
          toast.success("Vendor created"); // S7 create: navigate to the record page
          navigate({ to: "/vendors/$vendorId", params: { vendorId: saved.id } });
        }
      } catch (error) {
        // S4: keep every value; map attributable errors to fields, the rest to the summary.
        if (error instanceof ApiError && error.fieldErrors) {
          for (const [name, message] of Object.entries(error.fieldErrors)) {
            form.setError(name as VendorFieldName, { type: "server", message });
          }
        } else {
          setFormError(error instanceof ApiError ? error.message : "Something went wrong on our side. Try again.");
        }
        requestAnimationFrame(() => summaryRef.current?.focus());
      }
    },
    () => requestAnimationFrame(() => summaryRef.current?.focus()), // client errors → focus summary
  );

  if (isEdit && vendor.isPending) {
    // S1: skeleton sections with final geometry
    return (
      <div aria-hidden="true" className="mx-auto w-full max-w-[720px] space-y-6 px-4 py-6">
        <Skeleton className="h-7 w-48" />
        {vendorSections.map((section) => (
          <Card key={section.id}>
            <CardContent className="space-y-4">
              <Skeleton className="h-5 w-40" />
              {section.fields.map((name) => (
                <div key={name} className="space-y-2">
                  <Skeleton className="h-4 w-28" />
                  <Skeleton className="h-9 w-full" />
                </div>
              ))}
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  if (isEdit && vendor.isError) {
    return (
      <div className="mx-auto w-full max-w-[720px] px-4 py-6">
        <Alert variant="destructive" role="status">
          <CircleAlert aria-hidden="true" />
          <AlertTitle>Couldn't load this vendor</AlertTitle>
          <AlertDescription>
            <Button variant="outline" size="sm" className="mt-2" onClick={() => vendor.refetch()}>
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      </div>
    );
  }

  const title = isEdit ? (vendor.data?.legalName ?? "Edit vendor") : "New vendor";

  return (
    <div className="relative mx-auto flex w-full max-w-[1440px] gap-12 px-4 pt-6 lg:px-8">
      {/* Section index: wide screens only and only because there are >= 5 sections */}
      <nav aria-label="Form sections" className="sticky top-20 hidden h-fit w-48 shrink-0 wide:block">
        <ol className="space-y-1 text-sm">
          {vendorSections.map((section) => {
            const hasError = section.fields.some((name) => errors[name]);
            return (
              <li key={section.id}>
                <a
                  href={`#section-${section.id}`}
                  className={cn("block rounded-md px-2 py-1 text-muted-foreground hover:bg-muted hover:text-foreground", hasError && "text-destructive")}
                >
                  {section.title}
                  {hasError ? <span className="sr-only"> (has errors)</span> : null}
                </a>
              </li>
            );
          })}
        </ol>
      </nav>

      <div className="mx-auto flex w-full max-w-[720px] flex-col gap-6">
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbLink asChild>
                <Link to="/vendors">Vendors</Link>
              </BreadcrumbLink>
            </BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              <BreadcrumbPage>{title}</BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>

        {/* org-page-header without primary: the sticky footer owns the primary action */}
        <header className="space-y-1">
          <h1 className="text-xl font-semibold tracking-tight">{title}</h1>
          <p className="text-sm text-muted-foreground">
            {isEdit ? "Changes apply to future purchase orders." : "Add a vendor so your team can raise purchase orders."}
          </p>
        </header>

        {isReadOnly ? (
          // S5 read-only rendering with explanation
          <Alert role="status">
            <Lock aria-hidden="true" />
            <AlertTitle>You can view this vendor but not edit it</AlertTitle>
            <AlertDescription>Procurement admins can change vendor details.</AlertDescription>
          </Alert>
        ) : null}

        <form id="vendor-form" noValidate onSubmit={onSubmit} className="flex flex-col gap-6 pb-28">
          <ErrorSummary errors={errors} formError={formError} summaryRef={summaryRef} />

          {/* Submit pending: fields read-only via disabled fieldset; values preserved */}
          <fieldset disabled={isSubmitting} className="contents">
            {vendorSections.map((section) => (
              <Card key={section.id} id={`section-${section.id}`} className="scroll-mt-20">
                <CardContent>
                  {/* org-sectioned-form: fieldset + legend, heading + 1-line description, single column */}
                  <FieldSet>
                    <FieldLegend>{section.title}</FieldLegend>
                    <FieldDescription>{section.description}</FieldDescription>
                    <FieldGroup>
                      {section.id === "address" ? (
                        <>
                          <VendorField form={form} name="street" readOnly={isReadOnly} />
                          {/* Tightly coupled short pair shares a row */}
                          <div className="grid gap-4 sm:grid-cols-[1fr_auto]">
                            <VendorField form={form} name="city" readOnly={isReadOnly} />
                            <VendorField form={form} name="postalCode" readOnly={isReadOnly} />
                          </div>
                          <VendorField form={form} name="country" readOnly={isReadOnly} />
                        </>
                      ) : (
                        section.fields.map((name) => <VendorField key={name} form={form} name={name} readOnly={isReadOnly} />)
                      )}
                    </FieldGroup>
                  </FieldSet>
                </CardContent>
              </Card>
            ))}
          </fieldset>
        </form>
      </div>

      {/* Sticky footer action bar: Cancel + primary; "Unsaved changes" indicator while dirty */}
      {!isReadOnly ? (
        <div className="fixed inset-x-0 bottom-0 z-20 border-t border-border bg-background/95 backdrop-blur lg:left-[var(--sidebar-width,0px)]">
          <div className="mx-auto flex max-w-[720px] items-center justify-between gap-4 px-4 py-3">
            <p role="status" className="text-sm text-muted-foreground">
              {isDirty ? "Unsaved changes" : savedAt ? "Saved" : ""}
            </p>
            <div className="flex flex-col-reverse gap-2 sm:flex-row">
              <Button variant="outline" asChild>
                <Link to={isEdit ? "/vendors/$vendorId" : "/vendors"} params={vendorId ? { vendorId } : undefined}>
                  Cancel
                </Link>
              </Button>
              <Button type="submit" form="vendor-form" disabled={isSubmitting} className="min-w-36">
                {isSubmitting ? <Spinner /> : null}
                {isSubmitting ? "Saving…" : isEdit ? "Save changes" : "Create vendor"}
              </Button>
            </div>
          </div>
        </div>
      ) : null}

      {/* AC-LONG-3 dirty guard for in-app navigation */}
      <AlertDialog open={blocker.status === "blocked"}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Discard changes?</AlertDialogTitle>
            <AlertDialogDescription>Your edits to this vendor haven't been saved and will be lost.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => blocker.reset?.()}>Keep editing</AlertDialogCancel>
            <AlertDialogAction onClick={() => blocker.proceed?.()}>Discard</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

// =============================================================================
// Part B — tpl-form-short
//   Dialog: <= 6 fields, no background reference needed.
//   Sheet:  <= 12 fields, background reference needed.
// =============================================================================

export const contactFormSchema = z.object({
  fullName: z.string().trim().min(2, "Enter the contact's full name."),
  email: z.email("Enter an email address like name@example.com."),
  role: z.enum(["sales", "billing", "support", "other"], { error: "Choose a role." }),
  phone: z.string().trim().optional(),
});

export type ContactFormValues = z.infer<typeof contactFormSchema>;

interface VendorContact extends ContactFormValues {
  id: string;
  vendorId: string;
}

function useCreateContact(vendorId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (values: ContactFormValues) => apiClient.post<VendorContact>(`/vendors/${vendorId}/contacts`, values),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [...vendorKeys.detail(vendorId), "contacts"] }),
  });
}

/** Shared body for dialog and sheet variants: org-sectioned-form variant single-section. */
function ContactFields({ form, autoFocusFirst }: { form: UseFormReturn<ContactFormValues>; autoFocusFirst?: boolean }) {
  return (
    <FieldGroup>
      <Controller
        control={form.control}
        name="fullName"
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor="contact-full-name">Full name</FieldLabel>
            {/* AC-SHORT-1: first field autofocused on open */}
            <Input {...field} id="contact-full-name" autoFocus={autoFocusFirst} autoComplete="name" aria-invalid={fieldState.invalid} />
            <FieldError errors={[fieldState.error]} />
          </Field>
        )}
      />
      <Controller
        control={form.control}
        name="email"
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor="contact-email">Email</FieldLabel>
            <Input {...field} id="contact-email" type="email" autoComplete="email" aria-invalid={fieldState.invalid} />
            <FieldError errors={[fieldState.error]} />
          </Field>
        )}
      />
      <Controller
        control={form.control}
        name="role"
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor="contact-role">Role</FieldLabel>
            <Select value={field.value} onValueChange={field.onChange}>
              <SelectTrigger id="contact-role" className="w-full" aria-invalid={fieldState.invalid} onBlur={field.onBlur}>
                <SelectValue placeholder="Choose a role" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="sales">Sales</SelectItem>
                <SelectItem value="billing">Billing</SelectItem>
                <SelectItem value="support">Support</SelectItem>
                <SelectItem value="other">Other</SelectItem>
              </SelectContent>
            </Select>
            <FieldError errors={[fieldState.error]} />
          </Field>
        )}
      />
      <Controller
        control={form.control}
        name="phone"
        render={({ field }) => (
          <Field>
            <FieldLabel htmlFor="contact-phone">
              Phone <span className="font-normal text-muted-foreground">(optional)</span>
            </FieldLabel>
            <Input {...field} id="contact-phone" type="tel" autoComplete="tel" className="tabular-nums" />
          </Field>
        )}
      />
    </FieldGroup>
  );
}

/** Dirty guard shared by dialog and sheet: AC-SHORT-2 (Escape on clean closes; on dirty prompts). */
function useDirtyCloseGuard(isDirty: boolean, onOpenChange: (open: boolean) => void) {
  const [confirmOpen, setConfirmOpen] = React.useState(false);
  const requestOpenChange = (open: boolean) => {
    if (!open && isDirty) {
      setConfirmOpen(true);
      return;
    }
    onOpenChange(open);
  };
  const guard = (
    <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Discard changes?</AlertDialogTitle>
          <AlertDialogDescription>The information you entered will be lost.</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Keep editing</AlertDialogCancel>
          <AlertDialogAction
            onClick={() => {
              setConfirmOpen(false);
              onOpenChange(false);
            }}
          >
            Discard
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
  return { requestOpenChange, guard };
}

export function CreateContactDialog({
  vendorId,
  open,
  onOpenChange,
  onCreated,
}: {
  vendorId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: (contact: VendorContact) => void; // parent highlights the new row in its list
}) {
  const create = useCreateContact(vendorId);
  const alertRef = React.useRef<HTMLDivElement>(null);
  const [submitError, setSubmitError] = React.useState<string>();
  const form = useForm<ContactFormValues>({
    resolver: zodResolver(contactFormSchema),
    defaultValues: { fullName: "", email: "", role: undefined as unknown as ContactFormValues["role"], phone: "" },
    mode: "onTouched",
  });
  const { requestOpenChange, guard } = useDirtyCloseGuard(form.formState.isDirty && !create.isPending, (next) => {
    if (!next) {
      form.reset();
      setSubmitError(undefined);
    }
    onOpenChange(next);
  });

  const onSubmit = form.handleSubmit(async (values) => {
    setSubmitError(undefined);
    try {
      const contact = await create.mutateAsync(values);
      form.reset();
      onOpenChange(false);
      onCreated(contact);
      // S7: close container + toast with "View"
      toast.success("Contact created", { action: { label: "View", onClick: () => onCreated(contact) } });
    } catch (error) {
      // AC-SHORT-3: keep values; map field errors or show the alert and focus it.
      if (error instanceof ApiError && error.fieldErrors) {
        for (const [name, message] of Object.entries(error.fieldErrors)) {
          form.setError(name as keyof ContactFormValues, { type: "server", message }, { shouldFocus: true });
        }
      } else {
        setSubmitError(error instanceof ApiError ? error.message : "Couldn't create the contact. Try again.");
        requestAnimationFrame(() => alertRef.current?.focus());
      }
    }
  });

  return (
    <>
      <Dialog open={open} onOpenChange={requestOpenChange}>
        {/* Mobile: full-screen dialog with footer pinned to the bottom */}
        <DialogContent className="flex flex-col sm:max-w-md max-sm:h-dvh max-sm:max-w-none max-sm:rounded-none">
          <DialogHeader>
            <DialogTitle>Create contact</DialogTitle>
            <DialogDescription>Add someone at this vendor who receives purchase orders.</DialogDescription>
          </DialogHeader>
          <form
            id="create-contact-form"
            noValidate
            onSubmit={onSubmit}
            className="flex-1"
            // Ctrl/Cmd+Enter submits from multiline inputs; Enter submits from single-line inputs natively.
            onKeyDown={(event) => {
              if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) onSubmit();
            }}
          >
            {submitError ? (
              <Alert ref={alertRef} variant="destructive" role="alert" tabIndex={-1} className="mb-4">
                <CircleAlert aria-hidden="true" />
                <AlertTitle>Contact not created</AlertTitle>
                <AlertDescription>{submitError}</AlertDescription>
              </Alert>
            ) : null}
            <fieldset disabled={create.isPending} className="contents">
              <ContactFields form={form} autoFocusFirst />
            </fieldset>
          </form>
          <DialogFooter>
            <Button variant="outline" onClick={() => requestOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" form="create-contact-form" disabled={create.isPending} className="min-w-32">
              {create.isPending ? <Spinner /> : null}
              {create.isPending ? "Creating…" : "Create contact"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      {guard}
    </>
  );
}

export function EditContactSheet({
  contact,
  open,
  onOpenChange,
}: {
  contact: VendorContact | undefined;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const update = useMutation({
    mutationFn: (values: ContactFormValues) => apiClient.patch<VendorContact>(`/contacts/${contact?.id}`, values),
    onSuccess: (saved) => queryClient.invalidateQueries({ queryKey: [...vendorKeys.detail(saved.vendorId), "contacts"] }),
  });
  const form = useForm<ContactFormValues>({
    resolver: zodResolver(contactFormSchema),
    values: contact, // S1 edit: values arrive with the record; primary disabled until then
    mode: "onTouched",
  });
  const { requestOpenChange, guard } = useDirtyCloseGuard(form.formState.isDirty && !update.isPending, onOpenChange);

  const onSubmit = form.handleSubmit(async (values) => {
    try {
      await update.mutateAsync(values);
      form.reset(values);
      onOpenChange(false);
      toast.success("Contact updated");
    } catch {
      toast.error("Couldn't update the contact", { action: { label: "Retry", onClick: () => onSubmit() } });
    }
  });

  return (
    <>
      <Sheet open={open} onOpenChange={requestOpenChange}>
        {/* Sheet layout contract: flex column, scrollable body, sticky header/footer (pcs-adapter 02 §2) */}
        <SheetContent side="right" className="flex flex-col gap-0 p-0 sm:max-w-lg">
          <SheetHeader className="border-b border-border px-6 py-4">
            <SheetTitle>Edit {contact?.fullName ?? "contact"}</SheetTitle>
            <SheetDescription>Changes apply to future purchase orders.</SheetDescription>
          </SheetHeader>
          <form id="edit-contact-form" noValidate onSubmit={onSubmit} className="flex-1 overflow-y-auto px-6 py-4">
            {contact ? (
              <fieldset disabled={update.isPending} className="contents">
                <ContactFields form={form} autoFocusFirst />
              </fieldset>
            ) : (
              <div aria-hidden="true" className="space-y-4">
                {Array.from({ length: 4 }).map((_, index) => (
                  <div key={index} className="space-y-2">
                    <Skeleton className="h-4 w-24" />
                    <Skeleton className="h-9 w-full" />
                  </div>
                ))}
              </div>
            )}
          </form>
          <SheetFooter className="flex-row justify-end gap-2 border-t border-border bg-muted/30 px-6 py-4">
            <Button variant="outline" onClick={() => requestOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" form="edit-contact-form" disabled={!contact || update.isPending}>
              {update.isPending ? <Spinner /> : null}
              {update.isPending ? "Saving…" : "Save changes"}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
      {guard}
    </>
  );
}

// -----------------------------------------------------------------------------
// S6 helpers
//   Dependency-list failure (e.g. owners) belongs to the one field that needs it: render
//   <FieldError> plus a "Retry" link button under that field; the rest of the form stays usable.
//   Autosave failure on long forms: the banner below, with a manual "Save" fallback.
// -----------------------------------------------------------------------------

export function AutosaveFailedBanner({ onSaveNow, isSaving }: { onSaveNow: () => void; isSaving: boolean }) {
  return (
    <Alert role="status" className="border-warning/50 bg-warning/10 [&>svg]:text-warning">
      <TriangleAlert aria-hidden="true" />
      <AlertTitle>Changes not saved. Retrying…</AlertTitle>
      <AlertDescription className="flex flex-wrap items-center gap-2">
        Your edits are kept on this page.
        <Button variant="outline" size="sm" onClick={onSaveNow} disabled={isSaving}>
          {isSaving ? <Spinner /> : null}
          Save now
        </Button>
      </AlertDescription>
    </Alert>
  );
}
