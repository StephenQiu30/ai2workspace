'use client';

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from 'react';

export interface AssistantCourseContext {
  courseId: string;
  courseTitle?: string;
}

interface AssistantContextValue {
  open: boolean;
  setOpen: (next: boolean) => void;
  openWithCourse: (ctx?: AssistantCourseContext) => void;
  courseContext: AssistantCourseContext | null;
  setCourseContext: (ctx: AssistantCourseContext | null) => void;
}

const AssistantContext = createContext<AssistantContextValue | null>(null);

export function AssistantProvider({children}: {children: ReactNode}) {
  const [open, setOpen] = useState(false);
  const [courseContext, setCourseContext] =
    useState<AssistantCourseContext | null>(null);

  const openWithCourse = useCallback((ctx?: AssistantCourseContext) => {
    if (ctx) {
      setCourseContext(ctx);
    }
    setOpen(true);
  }, []);

  const value = useMemo<AssistantContextValue>(
    () => ({
      open,
      setOpen,
      openWithCourse,
      courseContext,
      setCourseContext,
    }),
    [open, openWithCourse, courseContext],
  );

  return (
    <AssistantContext.Provider value={value}>
      {children}
    </AssistantContext.Provider>
  );
}

export function useAssistantContext(): AssistantContextValue {
  const ctx = useContext(AssistantContext);
  if (!ctx) {
    throw new Error(
      'useAssistantContext must be used within AssistantProvider',
    );
  }
  return ctx;
}
