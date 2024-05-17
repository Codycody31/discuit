const initialState = {
  stats: {
    users: 0,
    posts: 0,
    comments: 0,
  },
  recentUsers: [],
  recentPosts: [],
  recentComments: [],
};

const typeStatsFetched = 'admin/statsFetched';
const typeRecentItemsFetched = 'admin/recentItemsFetched';

export default function adminReducer(state = initialState, action) {
  switch (action.type) {
    case typeStatsFetched: {
      return {
        ...state,
        stats: action.payload,
      };
    }
    case typeRecentItemsFetched: {
      return {
        ...state,
        recentUsers: action.payload.users,
        recentPosts: action.payload.posts,
        recentComments: action.payload.comments,
      };
    }
    default:
      return state;
  }
}

export const statsFetched = (stats) => {
  return { type: typeStatsFetched, payload: stats };
};

export const recentItemsFetched = (recentItems) => {
  return { type: typeRecentItemsFetched, payload: recentItems };
};

export const fetchStats = () => {
  return async (dispatch) => {
    // Placeholder for fetching stats
    // const response = await fetch('/api/admin/stats');
    // const data = await response.json();
    const data = { users: 100, posts: 200, comments: 300 }; // Dummy data
    dispatch(statsFetched(data));
  };
};

export const fetchRecentItems = () => {
  return async (dispatch) => {
    // Placeholder for fetching recent items
    // const response = await fetch('/api/admin/recent');
    // const data = await response.json();
    const data = {
      users: [
        { id: 1, name: 'User One' },
        { id: 2, name: 'User Two' },
      ],
      posts: [
        { id: 1, title: 'Post One' },
        { id: 2, title: 'Post Two' },
      ],
      comments: [
        { id: 1, content: 'Comment One' },
        { id: 2, content: 'Comment Two' },
      ],
    }; // Dummy data
    dispatch(recentItemsFetched(data));
  };
};
