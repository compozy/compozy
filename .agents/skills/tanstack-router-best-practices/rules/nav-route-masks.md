# nav-route-masks: Use Route Masks for Modal URLs

## Priority: LOW

## Explanation

Route masks let you display one URL while internally routing to another. This is useful for modals, sheets, and overlays where you want a shareable URL that shows the modal, but navigating there directly should show the full page.

## Bad Example

```tsx
// Modal without proper URL handling
function PostList() {
  const [selectedPost, setSelectedPost] = useState<string | null>(null);

  return (
    <div>
      {posts.map(post => (
        <div key={post.id} onClick={() => setSelectedPost(post.id)}>
          {post.title}
        </div>
      ))}

      {selectedPost && (
        <Modal onClose={() => setSelectedPost(null)}>
          <PostDetail postId={selectedPost} />
        </Modal>
      )}
    </div>
  );
}

// Problems:
// - URL doesn't change when modal opens
// - Can't share link to modal
// - Back button doesn't close modal
// - Refresh loses modal state
```

## Good Example: Route Masks for Modal

```tsx
// routes/posts.tsx
export const Route = createFileRoute("/posts")({
  component: PostList,
});

function PostList() {
  const posts = usePosts();

  return (
    <div>
      {posts.map(post => (
        <Link
          key={post.id}
          to="/posts/$postId"
          params={{ postId: post.id }}
          mask={{
            to: "/posts",
            // URL shows /posts but routes to /posts/$postId
          }}
        >
          {post.title}
        </Link>
      ))}
      <Outlet /> {/* Modal renders here */}
    </div>
  );
}

// routes/posts/$postId.tsx
export const Route = createFileRoute("/posts/$postId")({
  component: PostModal,
});

function PostModal() {
  const { postId } = Route.useParams();
  const navigate = useNavigate();

  return (
    <Modal onClose={() => navigate({ to: "/posts" })}>
      <PostDetail postId={postId} />
    </Modal>
  );
}

// User clicks post:
// - URL stays /posts (masked)
// - PostModal renders
// - Share link goes to /posts/$postId (real URL)
// - Direct navigation to /posts/$postId shows full page (no mask)
```

## Good Example: With Search Params

```tsx
function PostList() {
  return (
    <div>
      {posts.map(post => (
        <Link
          key={post.id}
          to="/posts/$postId"
          params={{ postId: post.id }}
          mask={{
            to: "/posts",
            search: { modal: post.id }, // /posts?modal=123
          }}
        >
          {post.title}
        </Link>
      ))}
    </div>
  );
}
```

## Good Example: Programmatic Navigation with Mask

```tsx
function PostCard({ post }: { post: Post }) {
  const navigate = useNavigate();

  const openInModal = () => {
    navigate({
      to: "/posts/$postId",
      params: { postId: post.id },
      mask: {
        to: "/posts",
      },
    });
  };

  const openFullPage = () => {
    navigate({
      to: "/posts/$postId",
      params: { postId: post.id },
      // No mask - shows real URL
    });
  };

  return (
    <div>
      <h3>{post.title}</h3>
      <button onClick={openInModal}>Quick View</button>
      <button onClick={openFullPage}>Full Page</button>
    </div>
  );
}
```

## Good Example: Unmask on Interaction

```tsx
function PostModal() {
  const { postId } = Route.useParams();
  const navigate = useNavigate();

  const expandToFullPage = () => {
    // Navigate to real URL, removing mask
    navigate({
      to: "/posts/$postId",
      params: { postId },
      // No mask = real URL
      replace: true, // Replace history entry
    });
  };

  return (
    <Modal>
      <PostDetail postId={postId} />
      <button onClick={expandToFullPage}>Expand to full page</button>
    </Modal>
  );
}
```

## Route Mask Behavior

| Scenario          | URL Shown             | Actual Route   |
| ----------------- | --------------------- | -------------- |
| Click masked link | Masked URL            | Real route     |
| Share/copy URL    | Real URL              | Real route     |
| Direct navigation | Real URL              | Real route     |
| Browser refresh   | Depends on URL in bar | Matches URL    |
| Back button       | Previous URL          | Previous route |

## Context

- Masks are client-side only - shared URLs are the real route
- Direct navigation to real URL bypasses mask (shows full page)
- Back button navigates through history correctly
- Use for modals, side panels, quick views
- Masks can include different search params
- Consider UX: users expect shared URLs to work
