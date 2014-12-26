package io.v.jenkins.plugins.vanadium_scm;

import static hudson.Util.fixEmpty;

import org.apache.commons.lang.time.FastDateFormat;
import org.kohsuke.stapler.export.Exported;
import org.kohsuke.stapler.export.ExportedBean;

import hudson.MarkupText;
import hudson.model.User;
import hudson.scm.ChangeLogAnnotator;
import hudson.scm.EditType;
import hudson.scm.ChangeLogSet;
import hudson.scm.ChangeLogSet.AffectedFile;

import java.io.IOException;
import java.text.ParseException;
import java.text.SimpleDateFormat;
import java.util.Collection;
import java.util.Comparator;
import java.util.Date;
import java.util.HashSet;
import java.util.List;
import java.util.TreeSet;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * Represents a change set.
 *
 * @author Nigel Magnay
 */
public class GitChangeSet extends ChangeLogSet.Entry {

  private static final String PREFIX_AUTHOR = "author ";
  private static final String PREFIX_COMMITTER = "committer ";
  private static final String IDENTITY = "([^<]*)<(.*)> (.*)";

  private static final Pattern FILE_LOG_ENTRY = Pattern.compile(
      "^:[0-9]{6} [0-9]{6} ([0-9a-f]{40}) ([0-9a-f]{40}) ([ACDMRTUX])(?>[0-9]+)?\t(.*)$");
  private static final Pattern AUTHOR_ENTRY = Pattern.compile("^" + PREFIX_AUTHOR + IDENTITY + "$");
  private static final Pattern COMMITTER_ENTRY =
      Pattern.compile("^" + PREFIX_COMMITTER + IDENTITY + "$");
  private static final Pattern RENAME_SPLIT = Pattern.compile("^(.*?)\t(.*)$");

  private static final String NULL_HASH = "0000000000000000000000000000000000000000";
  private static final String ISO_8601 = "yyyy-MM-dd'T'HH:mm:ssZ";

  /**
   * This is broken as a part of the 1.5 refactoring.
   *
   * <p>
   * When we build a commit that multiple branches point to, Git plugin historically recorded
   * changelogs "revOfBranchInPreviousBuild...revToBuild" for each branch separately. This however
   * fails to take full generality of Git commit graph into account, as such rev-lists can share
   * common commits, which then get reported multiple times.
   *
   * <p>
   * In Git, a commit doesn't belong to a branch, in the sense that you cannot look at the object
   * graph and re-construct exactly how branch has evolved. In that sense, trying to attribute
   * commits to branches is a somewhat futile exercise.
   *
   * <p>
   * On the other hand, if this is still deemed important, the right thing to do is to traverse the
   * commit graph and see if a commit can be only reachable from the "revOfBranchInPreviousBuild" of
   * just one branch, in which case it's safe to attribute the commit to that branch.
   */
  private String committer;
  private String committerEmail;
  private String committerTime;
  private String author;
  private String authorEmail;
  private String authorTime;
  private String comment;
  private String title;
  private String id;
  private String parentCommit;
  private Collection<Path> paths = new TreeSet<Path>(new Comparator<Path>() {
    @Override
    public int compare(Path o1, Path o2) {
      return o1.getPath().compareTo(o2.path);
    }
  });
  private boolean authorOrCommitter;

  /**
   * Create Git change set using information in given lines
   *
   * @param lines
   * @param authorOrCommitter
   */
  public GitChangeSet(List<String> lines, boolean authorOrCommitter) {
    this.authorOrCommitter = authorOrCommitter;
    if (lines.size() > 0) {
      parseCommit(lines);
    }
  }

  private void parseCommit(List<String> lines) {

    StringBuilder message = new StringBuilder();

    for (String line : lines) {
      if (line.length() < 1) {
        continue;
      }
      if (line.startsWith("commit ")) {
        this.id = line.split(" ")[1];
      } else if (line.startsWith("tree ")) {
      } else if (line.startsWith("parent ")) {
        this.parentCommit = line.split(" ")[1];
      } else if (line.startsWith(PREFIX_COMMITTER)) {
        Matcher committerMatcher = COMMITTER_ENTRY.matcher(line);
        if (committerMatcher.matches() && committerMatcher.groupCount() >= 3) {
          this.committer = committerMatcher.group(1).trim();
          this.committerEmail = committerMatcher.group(2);
          this.committerTime = isoDateFormat(committerMatcher.group(3));
        }
      } else if (line.startsWith(PREFIX_AUTHOR)) {
        Matcher authorMatcher = AUTHOR_ENTRY.matcher(line);
        if (authorMatcher.matches() && authorMatcher.groupCount() >= 3) {
          this.author = authorMatcher.group(1).trim();
          this.authorEmail = authorMatcher.group(2);
          this.authorTime = isoDateFormat(authorMatcher.group(3));
        }
      } else if (line.startsWith("    ")) {
        message.append(line.substring(4)).append('\n');
      } else if (':' == line.charAt(0)) {
        Matcher fileMatcher = FILE_LOG_ENTRY.matcher(line);
        if (fileMatcher.matches() && fileMatcher.groupCount() >= 4) {
          String mode = fileMatcher.group(3);
          if (mode.length() == 1) {
            String src = null;
            String dst = null;
            String path = fileMatcher.group(4);
            char editMode = mode.charAt(0);
            if (editMode == 'M' || editMode == 'A' || editMode == 'D' || editMode == 'R'
                || editMode == 'C') {
              src = parseHash(fileMatcher.group(1));
              dst = parseHash(fileMatcher.group(2));
            }

            // Handle rename as two operations - a delete and an add
            if (editMode == 'R') {
              Matcher renameSplitMatcher = RENAME_SPLIT.matcher(path);
              if (renameSplitMatcher.matches() && renameSplitMatcher.groupCount() >= 2) {
                String oldPath = renameSplitMatcher.group(1);
                String newPath = renameSplitMatcher.group(2);
                this.paths.add(new Path(src, dst, 'D', oldPath, this));
                this.paths.add(new Path(src, dst, 'A', newPath, this));
              }
            }
            // Handle copy as an add
            else if (editMode == 'C') {
              Matcher copySplitMatcher = RENAME_SPLIT.matcher(path);
              if (copySplitMatcher.matches() && copySplitMatcher.groupCount() >= 2) {
                String newPath = copySplitMatcher.group(2);
                this.paths.add(new Path(src, dst, 'A', newPath, this));
              }
            } else {
              this.paths.add(new Path(src, dst, editMode, path, this));
            }
          }
        }
      }
    }

    this.comment = message.toString();

    int endOfFirstLine = this.comment.indexOf('\n');
    if (endOfFirstLine == -1) {
      this.title = this.comment;
    } else {
      this.title = this.comment.substring(0, endOfFirstLine);
    }
  }

  /** Convert to iso date format if required */
  private String isoDateFormat(String s) {
    int i = s.indexOf(' ');
    long time = Long.parseLong(s.substring(0, i));
    return FastDateFormat.getInstance(ISO_8601).format(new Date(time * 1000)) + s.substring(i);
  }

  private String parseHash(String hash) {
    return NULL_HASH.equals(hash) ? null : hash;
  }

  @Exported
  public String getDate() {
    return authorOrCommitter ? authorTime : committerTime;
  }

  @Override
  public long getTimestamp() {
    try {
      return new SimpleDateFormat(ISO_8601).parse(getDate()).getTime();
    } catch (ParseException e) {
      return -1;
    }
  }

  @Override
  public String getCommitId() {
    return id;
  }

  @Override
  public void setParent(ChangeLogSet parent) {
    super.setParent(parent);
  }

  public String getParentCommit() {
    return parentCommit;
  }


  @Override
  public Collection<String> getAffectedPaths() {
    Collection<String> affectedPaths = new HashSet<String>(this.paths.size());
    for (Path file : this.paths) {
      affectedPaths.add(file.getPath());
    }
    return affectedPaths;
  }

  /**
   * Gets the files that are changed in this commit.
   *
   * @return can be empty but never null.
   */
  @Exported
  public Collection<Path> getPaths() {
    return paths;
  }

  @Override
  public Collection<Path> getAffectedFiles() {
    return this.paths;
  }

  /**
   * Returns user of the change set.
   *
   * @param csAuthor user name.
   * @param csAuthorEmail user email.
   * @param createAccountBasedOnEmail true if create new user based on committer's email.
   * @return {@link User}
   */
  public User findOrCreateUser(String csAuthor, String csAuthorEmail,
      boolean createAccountBasedOnEmail) {
    User user;
    if (csAuthor == null) {
      return User.getUnknown();
    }
    if (createAccountBasedOnEmail) {
      user = User.get(csAuthorEmail, false);

      if (user == null) {
        try {
          user = User.get(csAuthorEmail, true);
          user.setFullName(csAuthor);
          if (hasHudsonTasksMailer()) {
            setMail(user, csAuthorEmail);
          }
          user.save();
        } catch (IOException e) {
          // add logging statement?
        }
      }
    } else {
      user = User.get(csAuthor, false);

      if (user == null) {
        user = User.get(csAuthorEmail.split("@")[0], true);
      }
    }
    // set email address for user if none is already available
    if (fixEmpty(csAuthorEmail) != null && hasHudsonTasksMailer() && !hasMail(user)) {
      try {
        setMail(user, csAuthorEmail);
      } catch (IOException e) {
        // ignore
      }
    }
    return user;
  }

  private void setMail(User user, String csAuthorEmail) throws IOException {
    user.addProperty(new hudson.tasks.Mailer.UserProperty(csAuthorEmail));
  }

  private boolean hasMail(User user) {
    hudson.tasks.Mailer.UserProperty property =
        user.getProperty(hudson.tasks.Mailer.UserProperty.class);
    return property != null && property.hasExplicitlyConfiguredAddress();
  }

  private boolean hasHudsonTasksMailer() {
    // TODO convert to checking for mailer plugin as plugin migrates to 1.509+
    try {
      Class.forName("hudson.tasks.Mailer");
      return true;
    } catch (ClassNotFoundException e) {
      return false;
    }
  }

  private boolean isCreateAccountBasedOnEmail() {
    return true;
  }

  @Override
  @Exported
  public User getAuthor() {
    String csAuthor;
    String csAuthorEmail;

    // If true, use the author field from git log rather than the committer.
    if (authorOrCommitter) {
      csAuthor = this.author;
      csAuthorEmail = this.authorEmail;
    } else {
      csAuthor = this.committer;
      csAuthorEmail = this.committerEmail;
    }

    return findOrCreateUser(csAuthor, csAuthorEmail, isCreateAccountBasedOnEmail());
  }

  /**
   * Gets the author name for this changeset - note that this is mainly here so that we can test
   * authorOrCommitter without needing a fully instantiated Hudson (which is needed for User.get in
   * getAuthor()).
   *
   * @return author name
   */
  public String getAuthorName() {
    // If true, use the author field from git log rather than the committer.
    String csAuthor = authorOrCommitter ? author : committer;
    return csAuthor;
  }

  public String getAuthorLDAP() {
    return authorEmail.substring(0, authorEmail.indexOf("@"));
  }

  @Override
  @Exported
  public String getMsg() {
    return this.title;
  }

  @Exported
  public String getId() {
    return this.id;
  }

  public String getRevision() {
    return this.id;
  }

  @Exported
  public String getComment() {
    return this.comment;
  }

  /**
   * Gets {@linkplain #getComment() the comment} fully marked up by {@link ChangeLogAnnotator}.
   */
  public String getCommentAnnotated() {
    MarkupText markup = new MarkupText(getComment());
    for (ChangeLogAnnotator a : ChangeLogAnnotator.all())
      a.annotate(getParent().build, this, markup);

    return markup.toString(false);
  }

  public String getBranch() {
    return null;
  }

  @ExportedBean(defaultVisibility = 999)
  public static class Path implements AffectedFile {

    private String src;
    private String dst;
    private char action;
    private String path;
    private GitChangeSet changeSet;

    private Path(String source, String destination, char action, String filePath,
        GitChangeSet changeSet) {
      this.src = source;
      this.dst = destination;
      this.action = action;
      this.path = filePath;
      this.changeSet = changeSet;
    }

    public String getSrc() {
      return src;
    }

    public String getDst() {
      return dst;
    }

    @Override
    @Exported(name = "file")
    public String getPath() {
      return path;
    }

    public GitChangeSet getChangeSet() {
      return changeSet;
    }

    @Override
    @Exported
    public EditType getEditType() {
      switch (action) {
        case 'A':
          return EditType.ADD;
        case 'D':
          return EditType.DELETE;
        default:
          return EditType.EDIT;
      }
    }
  }

  @Override
  public int hashCode() {
    return id != null ? id.hashCode() : super.hashCode();
  }

  @Override
  public boolean equals(Object obj) {
    if (obj == this) {
      return true;
    }
    if (obj instanceof GitChangeSet) {
      return id != null && id.equals(((GitChangeSet) obj).id);
    }
    return false;
  }
}
