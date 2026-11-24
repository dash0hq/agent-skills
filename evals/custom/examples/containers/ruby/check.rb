# frozen_string_literal: true

# Static check of the Ruby example files listed in the file named by the first
# argument (see evals/custom/examples/ruby.go). It runs under `bundle exec`
# with the pinned bundle and never executes example code:
#
#  1. `ruby -wc` parses each file; a syntax error fails the file.
#  2. Every top-level `require "<library>"` with a literal argument is
#     required, which loads library code only; a LoadError fails the file.
#  3. Every constant path whose root constant is defined after those requires,
#     for example OpenTelemetry::Trace::Status, must resolve.
#  4. Every method called on such a constant, for example
#     OpenTelemetry::Trace::Status.error(...), must exist and accept the
#     call's positional and keyword arguments, unless the file guards it with
#     `Const.respond_to?(:method)`.
#
# Names whose root is not defined by a required library (application state
# such as `order`, `logger`, or `Rails`) are not checked. Each file is checked
# in a forked process, so the requires of one file do not leak into another.
# Errors print as "<path>:<line>:<column>: error: <message>".

require "open3"
require "prism"
require "rbconfig"

module RubyExampleCheck
  module_function

  def error(path, location, message)
    puts "#{path}:#{location.start_line}:#{location.start_column + 1}: error: #{message}"
  end

  # Returns the "A::B::C" name of a constant read or path node, or nil when a
  # segment is dynamic (for example `foo::Bar`).
  def constant_name(node)
    case node
    when Prism::ConstantReadNode
      node.name.to_s
    when Prism::ConstantPathNode
      return node.name.to_s if node.parent.nil?

      parent = constant_name(node.parent)
      parent && "#{parent}::#{node.name}"
    end
  end

  # Resolves a constant name from the top level; returns [value, nil] or
  # [nil, error message]. A segment under a non-module value stops resolution
  # (nothing to check).
  def resolve(name)
    value = Object
    name.split("::").each do |segment|
      return [nil, nil] unless value.is_a?(Module)

      begin
        value = value.const_get(segment)
      rescue NameError
        return [nil, "uninitialized constant #{name}"]
      end
    end
    [value, nil]
  end

  # Collects the constants a file defines (class, module, and constant
  # assignments), whose references are the example's own code, and the
  # `Const.respond_to?(:name)` guards, whose guarded calls are not checked.
  class Definitions < Prism::Visitor
    attr_reader :roots, :guarded

    def initialize
      @roots = {}
      @guarded = {}
      super
    end

    def visit_call_node(node)
      args = node.arguments&.arguments || []
      if node.name == :respond_to? && args.first.is_a?(Prism::SymbolNode)
        name = RubyExampleCheck.constant_name(node.receiver)
        @guarded["#{name}.#{args.first.unescaped}"] = true if name
      end
      super
    end

    def visit_class_node(node)
      add(node.constant_path)
      super
    end

    def visit_module_node(node)
      add(node.constant_path)
      super
    end

    def visit_constant_write_node(node)
      @roots[node.name.to_s] = true
      super
    end

    def visit_constant_path_write_node(node)
      add(node.target)
      super
    end

    def add(path)
      name = RubyExampleCheck.constant_name(path)
      @roots[name.split("::").first] = true if name
    end
  end

  # Checks constant references and calls on constants.
  class References < Prism::Visitor
    attr_reader :failed

    def initialize(path, definitions)
      @path = path
      @own_roots = definitions.roots
      @guarded = definitions.guarded
      @failed = false
      super()
    end

    def visit_constant_read_node(node)
      check_constant(node)
    end

    def visit_constant_path_node(node)
      # The outermost path node covers its parents; recurse only into a
      # dynamic parent such as `foo::Bar`.
      return super unless RubyExampleCheck.constant_name(node)

      check_constant(node)
    end

    def visit_call_node(node)
      check_call(node) if node.receiver
      super
    end

    private

    def report(location, message)
      RubyExampleCheck.error(@path, location, message)
      @failed = true
    end

    # Returns the resolved value of a checked constant node, or nil.
    def check_constant(node)
      name = RubyExampleCheck.constant_name(node)
      return nil unless name

      root = name.split("::").first
      return nil if @own_roots[root] || !Object.const_defined?(root)

      value, message = RubyExampleCheck.resolve(name)
      report(node.location, message) if message
      value
    end

    def check_call(node)
      receiver = node.receiver
      return unless receiver.is_a?(Prism::ConstantReadNode) || receiver.is_a?(Prism::ConstantPathNode)

      name = RubyExampleCheck.constant_name(receiver)
      return unless name

      root = name.split("::").first
      return if @own_roots[root] || !Object.const_defined?(root)

      target, = RubyExampleCheck.resolve(name)
      return unless target.is_a?(Module)

      method_name = node.name
      return if @guarded["#{name}.#{method_name}"]

      unless target.respond_to?(method_name)
        report(node.message_loc || node.location, "undefined method '#{method_name}' for #{name}")
        return
      end
      check_arguments(node, target, method_name, name)
    end

    def check_arguments(node, target, method_name, name)
      method = target.method(method_name)
      parameters = method.parameters
      if method_name == :new && method.owner == Class && target.is_a?(Class)
        parameters = target.instance_method(:initialize).parameters
      end
      types = parameters.map(&:first)
      args = node.arguments&.arguments || []
      return if args.any? { |a| a.is_a?(Prism::SplatNode) || a.is_a?(Prism::ForwardingArgumentsNode) }

      keywords = args.last.is_a?(Prism::KeywordHashNode) ? args.last : nil
      accepts_keywords = types.any? { |t| %i[key keyreq keyrest].include?(t) }
      positional = keywords && accepts_keywords ? args.size - 1 : args.size

      required = types.count(:req)
      optional = types.count(:opt)
      unless types.include?(:rest) || positional.between?(required, required + optional)
        expected = optional.zero? ? required.to_s : "#{required}..#{required + optional}"
        report(node.message_loc || node.location, "wrong number of arguments for #{name}.#{method_name} (given #{positional}, expected #{expected})")
        return
      end
      return unless keywords && accepts_keywords

      check_keywords(node, keywords, parameters, "#{name}.#{method_name}")
    end

    def check_keywords(node, keywords, parameters, label)
      given = []
      keywords.elements.each do |element|
        return unless element.is_a?(Prism::AssocNode) && element.key.is_a?(Prism::SymbolNode)

        given << element.key.unescaped.to_sym
      end
      known = parameters.select { |t, _| %i[key keyreq].include?(t) }.map(&:last)
      unless parameters.any? { |t, _| t == :keyrest }
        unknown = given - known
        report(node.message_loc || node.location, "unknown keyword#{'s' if unknown.size > 1} for #{label}: #{unknown.join(', ')}") unless unknown.empty?
      end
      missing = parameters.select { |t, _| t == :keyreq }.map(&:last) - given
      report(node.message_loc || node.location, "missing keyword#{'s' if missing.size > 1} for #{label}: #{missing.join(', ')}") unless missing.empty?
    end
  end

  # Returns the literal library names of the top-level require calls.
  def requires(program)
    program.statements.body.filter_map do |statement|
      next unless statement.is_a?(Prism::CallNode) && statement.name == :require && statement.receiver.nil?

      args = statement.arguments&.arguments
      next unless args&.size == 1 && args.first.is_a?(Prism::StringNode)

      [args.first.unescaped, statement.location]
    end
  end

  def check_file(path)
    out, status = Open3.capture2e(RbConfig.ruby, "-wc", path)
    out.each_line { |line| puts line unless line.strip == "Syntax OK" }
    result = Prism.parse_file(path)
    unless status.success? && result.success?
      result.errors.each { |e| error(path, e.location, "syntax error: #{e.message}") }
      puts "#{path}:1:1: error: ruby -wc failed" if result.errors.empty?
      return false
    end

    ok = true
    requires(result.value).each do |library, location|
      require library
    rescue LoadError, StandardError, ScriptError => e
      error(path, location, "require '#{library}' failed: #{e.class}: #{e.message.lines.first&.strip}")
      ok = false
    end
    return false unless ok

    definitions = Definitions.new
    result.value.accept(definitions)
    references = References.new(path, definitions)
    result.value.accept(references)
    !references.failed
  end
end

failed = false
File.readlines(ARGV.fetch(0), chomp: true).reject(&:empty?).each do |path|
  pid = fork do
    $stdout.sync = true
    exit!(RubyExampleCheck.check_file(path) ? 0 : 1)
  end
  Process.wait(pid)
  failed ||= !$?.success?
end
exit(failed ? 1 : 0)
