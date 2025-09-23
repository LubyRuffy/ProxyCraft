import { jsx as _jsx } from "react/jsx-runtime";
import { cva } from 'class-variance-authority';
import { cn } from '@/lib/utils';
const badgeVariants = cva('inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2', {
    variants: {
        variant: {
            default: 'border-transparent bg-secondary text-secondary-foreground hover:bg-secondary/80',
            success: 'border-transparent bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
            warning: 'border-transparent bg-amber-500/10 text-amber-600 dark:text-amber-400',
            destructive: 'border-transparent bg-destructive/10 text-destructive dark:text-destructive-foreground',
            outline: 'text-foreground',
        },
    },
    defaultVariants: {
        variant: 'default',
    },
});
export function Badge({ className, variant, ...props }) {
    return _jsx("div", { className: cn(badgeVariants({ variant }), className), ...props });
}
