import { useState } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { Navigate, useNavigate } from "@tanstack/react-router";
import {
  AlertCircle,
  ArrowRight,
  Loader2,
  MessagesSquare,
  ShieldCheck,
  Sparkles,
} from "lucide-react";
import {
  loginSchema,
  registerSchema,
  type LoginInput,
  type RegisterInput,
} from "@chatfree/shared";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ThemeToggle } from "@/components/theme-toggle";

export function LoginPage() {
  const { session, loading, login, register } = useAuth();
  const navigate = useNavigate();
  const [mode, setMode] = useState<"login" | "register">("login");
  const [submitError, setSubmitError] = useState<string | null>(null);

  const loginForm = useForm<LoginInput>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
  });

  const registerForm = useForm<RegisterInput>({
    resolver: zodResolver(registerSchema),
    defaultValues: { name: "", workspaceName: "", email: "", password: "" },
  });

  if (loading) {
    return (
      <div className="flex min-h-svh items-center justify-center bg-background">
        <Loader2 className="size-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (session) {
    return <Navigate to="/" replace />;
  }

  const submitLogin = loginForm.handleSubmit(async (values) => {
    setSubmitError(null);
    try {
      await login(values.email, values.password);
      void navigate({ to: "/", replace: true });
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : "Não foi possível entrar.");
    }
  });

  const submitRegister = registerForm.handleSubmit(async (values) => {
    setSubmitError(null);
    try {
      await register(values);
      void navigate({ to: "/", replace: true });
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : "Não foi possível criar a conta.");
    }
  });

  const isSubmitting = loginForm.formState.isSubmitting || registerForm.formState.isSubmitting;

  const fillDemo = () => {
    setSubmitError(null);
    setMode("login");
    loginForm.setValue("email", "demo@chatfree.dev", { shouldValidate: true });
    loginForm.setValue("password", "demo1234", { shouldValidate: true });
  };

  return (
    <div className="relative grid min-h-svh bg-background lg:grid-cols-[1.05fr_1fr]">
      <div className="absolute right-4 top-4 z-20">
        <ThemeToggle className="size-9 border border-border/60 bg-card/70 shadow-xs backdrop-blur" />
      </div>
      <section className="relative hidden overflow-hidden bg-[oklch(0.21_0.03_268)] p-12 text-white lg:flex lg:flex-col lg:justify-between">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 bg-[radial-gradient(60%_50%_at_20%_10%,oklch(0.55_0.2_280/.55),transparent),radial-gradient(50%_40%_at_90%_80%,oklch(0.62_0.16_190/.35),transparent)]"
        />
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 opacity-[0.14] [background-image:linear-gradient(to_right,white_1px,transparent_1px),linear-gradient(to_bottom,white_1px,transparent_1px)] [background-size:44px_44px]"
        />

        <div className="relative flex items-center gap-2.5">
          <span className="flex size-10 items-center justify-center rounded-xl bg-white/10 ring-1 ring-white/15">
            <MessagesSquare className="size-5" />
          </span>
          <span className="text-sm font-semibold tracking-tight">ChatFree</span>
        </div>

        <div className="relative flex max-w-lg flex-col gap-6">
          <span className="w-fit rounded-full border border-white/15 bg-white/5 px-3 py-1 text-xs font-medium text-white/80">
            Atendimento humanizado, automação livre
          </span>
          <h1 className="text-4xl font-semibold leading-[1.1] tracking-tight">
            Conversas que começam no bot e terminam com pessoas.
          </h1>
          <p className="text-base leading-relaxed text-white/70">
            Construa fluxos no editor visual, incorpore o widget em qualquer site e assuma
            o atendimento em tempo real quando o visitante pedir.
          </p>
          <div className="grid gap-3 sm:grid-cols-2">
            <Feature icon={Sparkles} title="Fluxos visuais" description="Nodes conectados, sem código." />
            <Feature icon={ShieldCheck} title="Multi-tenant" description="Workspace, papéis e origens." />
          </div>
        </div>

        <p className="relative text-xs text-white/40">
          ChatFree · plataforma de atendimento conversacional
        </p>
      </section>

      <section className="flex items-center justify-center px-4 py-10">
        <div className="w-full max-w-md">
          <div className="mb-8 flex items-center gap-2.5 lg:hidden">
            <span className="flex size-9 items-center justify-center rounded-xl bg-primary text-primary-foreground">
              <MessagesSquare className="size-5" />
            </span>
            <span className="text-sm font-semibold">ChatFree</span>
          </div>

          <div
            role="tablist"
            aria-label="Entrar ou criar conta"
            className="mb-6 flex rounded-lg border border-border/70 bg-muted/40 p-1"
          >
            <button
              type="button"
              role="tab"
              aria-selected={mode === "login"}
              onClick={() => {
                setMode("login");
                setSubmitError(null);
              }}
              className={`flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors duration-fast ${
                mode === "login"
                  ? "bg-background text-foreground shadow-xs"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              Entrar
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={mode === "register"}
              onClick={() => {
                setMode("register");
                setSubmitError(null);
              }}
              className={`flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors duration-fast ${
                mode === "register"
                  ? "bg-background text-foreground shadow-xs"
                  : "text-muted-foreground hover:text-foreground"
              }`}
            >
              Criar conta
            </button>
          </div>

          <div className="mb-6">
            <h2 className="text-2xl font-semibold tracking-tight">
              {mode === "login" ? "Bem-vindo de volta" : "Comece agora"}
            </h2>
            <p className="mt-1 text-sm text-muted-foreground">
              {mode === "login"
                ? "Entre para acompanhar conversas e fluxos."
                : "Crie seu workspace e receba o primeiro widget em segundos."}
            </p>
          </div>

          {mode === "login" ? (
            <form onSubmit={submitLogin} className="flex flex-col gap-4">
              <FieldGroup>
                <Field data-invalid={Boolean(loginForm.formState.errors.email)}>
                  <FieldLabel htmlFor="login-email">E-mail</FieldLabel>
                  <Input
                    id="login-email"
                    type="email"
                    autoComplete="email"
                    placeholder="voce@empresa.com"
                    aria-invalid={Boolean(loginForm.formState.errors.email)}
                    {...loginForm.register("email")}
                  />
                  <FieldError errors={[loginForm.formState.errors.email]} />
                </Field>
                <Field data-invalid={Boolean(loginForm.formState.errors.password)}>
                  <FieldLabel htmlFor="login-password">Senha</FieldLabel>
                  <Input
                    id="login-password"
                    type="password"
                    autoComplete="current-password"
                    placeholder="••••••••"
                    aria-invalid={Boolean(loginForm.formState.errors.password)}
                    {...loginForm.register("password")}
                  />
                  <FieldError errors={[loginForm.formState.errors.password]} />
                </Field>
              </FieldGroup>

              {submitError ? (
                <p
                  role="alert"
                  className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm leading-snug text-destructive"
                >
                  <AlertCircle className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
                  {submitError}
                </p>
              ) : null}

              <Button type="submit" disabled={isSubmitting} className="w-full">
                {isSubmitting ? (
                  <Loader2 data-icon="inline-start" className="animate-spin" />
                ) : (
                  <ArrowRight data-icon="inline-start" />
                )}
                Entrar
              </Button>

              <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-dashed border-border/80 bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
                <span>
                  Demo: <span className="font-medium text-foreground">demo@chatfree.dev</span>
                  {" · "}
                  <span className="font-medium text-foreground">demo1234</span>
                </span>
                <button
                  type="button"
                  onClick={fillDemo}
                  className="font-medium text-primary underline-offset-4 transition-colors hover:underline"
                >
                  Preencher
                </button>
              </div>
            </form>
          ) : (
            <form onSubmit={submitRegister} className="flex flex-col gap-4">
              <FieldGroup>
                <Field data-invalid={Boolean(registerForm.formState.errors.name)}>
                  <FieldLabel htmlFor="register-name">Seu nome</FieldLabel>
                  <Input
                    id="register-name"
                    autoComplete="name"
                    placeholder="Maria Silva"
                    aria-invalid={Boolean(registerForm.formState.errors.name)}
                    {...registerForm.register("name")}
                  />
                  <FieldError errors={[registerForm.formState.errors.name]} />
                </Field>
                <Field data-invalid={Boolean(registerForm.formState.errors.workspaceName)}>
                  <FieldLabel htmlFor="register-workspace">Workspace</FieldLabel>
                  <Input
                    id="register-workspace"
                    placeholder="Minha empresa"
                    aria-invalid={Boolean(registerForm.formState.errors.workspaceName)}
                    {...registerForm.register("workspaceName")}
                  />
                  <FieldError errors={[registerForm.formState.errors.workspaceName]} />
                </Field>
                <Field data-invalid={Boolean(registerForm.formState.errors.email)}>
                  <FieldLabel htmlFor="register-email">E-mail</FieldLabel>
                  <Input
                    id="register-email"
                    type="email"
                    autoComplete="email"
                    placeholder="voce@empresa.com"
                    aria-invalid={Boolean(registerForm.formState.errors.email)}
                    {...registerForm.register("email")}
                  />
                  <FieldError errors={[registerForm.formState.errors.email]} />
                </Field>
                <Field data-invalid={Boolean(registerForm.formState.errors.password)}>
                  <FieldLabel htmlFor="register-password">Senha</FieldLabel>
                  <Input
                    id="register-password"
                    type="password"
                    autoComplete="new-password"
                    placeholder="Mínimo de 8 caracteres"
                    aria-invalid={Boolean(registerForm.formState.errors.password)}
                    {...registerForm.register("password")}
                  />
                  <FieldError errors={[registerForm.formState.errors.password]} />
                </Field>
              </FieldGroup>

              {submitError ? (
                <p
                  role="alert"
                  className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm leading-snug text-destructive"
                >
                  <AlertCircle className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
                  {submitError}
                </p>
              ) : null}

              <Button type="submit" disabled={isSubmitting} className="w-full">
                {isSubmitting ? (
                  <Loader2 data-icon="inline-start" className="animate-spin" />
                ) : (
                  <Sparkles data-icon="inline-start" />
                )}
                Criar workspace
              </Button>
            </form>
          )}
        </div>
      </section>
    </div>
  );
}

function Feature({
  icon: Icon,
  title,
  description,
}: {
  icon: typeof Sparkles;
  title: string;
  description: string;
}) {
  return (
    <div className="rounded-xl border border-white/10 bg-white/5 p-3 transition-colors duration-fast hover:bg-white/10">
      <Icon className="size-4 text-white/80" />
      <p className="mt-2 text-sm font-medium">{title}</p>
      <p className="mt-0.5 text-xs text-white/50">{description}</p>
    </div>
  );
}
