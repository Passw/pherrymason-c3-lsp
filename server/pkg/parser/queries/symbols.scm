(source_file [
        (module_declaration)    @module_dec
        (import_declaration)    @import_dec

        (global_declaration [
                (declaration)       @global_decl
                (const_declaration) @const_decl
                (func_declaration)  @function_dec
        ])

        (func_definition)       @function_def
        (alias_declaration)     @def_dec
        (typedef_declaration)   @distinct_dec
        (constdef_declaration)  @constdef_dec
        (attrdef_declaration)   @attrdef_dec
        (struct_declaration)    @struct_dec
        (bitstruct_declaration) @bitstruct_dec
        (enum_declaration)      @enum_dec
        (faultdef_declaration)  @fault_doc
        (interface_declaration) @interface_dec
        (macro_declaration)     @macro_dec
])
