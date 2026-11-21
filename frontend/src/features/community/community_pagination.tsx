import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from '@/components/ui/pagination';

export interface CommunityPaginationProps {
  currentPage: number;
  totalItems: number;
  pageSize: number;
  buildPageHref: (page: number) => string;
}

function buildPageNumbers(
  current: number,
  totalPages: number,
): Array<number | 'ellipsis'> {
  if (totalPages <= 7) {
    return Array.from({length: totalPages}, (_, idx) => idx + 1);
  }
  const pages: Array<number | 'ellipsis'> = [1];
  const start = Math.max(2, current - 1);
  const end = Math.min(totalPages - 1, current + 1);

  if (start > 2) {
    pages.push('ellipsis');
  }
  for (let p = start; p <= end; p += 1) {
    pages.push(p);
  }
  if (end < totalPages - 1) {
    pages.push('ellipsis');
  }
  pages.push(totalPages);
  return pages;
}

export function CommunityPagination({
  currentPage,
  totalItems,
  pageSize,
  buildPageHref,
}: CommunityPaginationProps) {
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize));
  if (totalPages <= 1) {
    return null;
  }

  const safeCurrent = Math.min(Math.max(1, currentPage), totalPages);
  const pageEntries = buildPageNumbers(safeCurrent, totalPages);

  return (
    <Pagination aria-label="分页导航" className="pt-4">
      <PaginationContent className="gap-1">
        <PaginationItem>
          {safeCurrent > 1 ? (
            <PaginationPrevious
              text="上一页"
              href={buildPageHref(safeCurrent - 1)}
            />
          ) : (
            <PaginationPrevious
              text="上一页"
              aria-disabled="true"
              tabIndex={-1}
              className="pointer-events-none opacity-40"
            />
          )}
        </PaginationItem>

        {pageEntries.map((entry, idx) =>
          entry === 'ellipsis' ? (
            <PaginationItem key={`ellipsis-${idx}`}>
              <PaginationEllipsis />
            </PaginationItem>
          ) : (
            <PaginationItem key={entry}>
              <PaginationLink
                href={buildPageHref(entry)}
                isActive={entry === safeCurrent}
                className={
                  entry === safeCurrent
                    ? 'border-transparent bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground'
                    : undefined
                }
              >
                {entry}
              </PaginationLink>
            </PaginationItem>
          ),
        )}

        <PaginationItem>
          {safeCurrent < totalPages ? (
            <PaginationNext
              text="下一页"
              href={buildPageHref(safeCurrent + 1)}
            />
          ) : (
            <PaginationNext
              text="下一页"
              aria-disabled="true"
              tabIndex={-1}
              className="pointer-events-none opacity-40"
            />
          )}
        </PaginationItem>
      </PaginationContent>
    </Pagination>
  );
}
